package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vintemhq/vintem/internal/adapters/mcp"
	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// conectarMCP liga um cliente MCP ao servidor do Vintém em memória.
func conectarMCP(t *testing.T, api mcp.API) *sdk.ClientSession {
	t.Helper()
	ctx := context.Background()
	lado1, lado2 := sdk.NewInMemoryTransports()
	srv, err := mcp.NovoServidor(api, "v-test").Connect(ctx, lado1, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	cs, err := sdk.NewClient(&sdk.Implementation{Name: "teste", Version: "1"}, nil).Connect(ctx, lado2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// ferramenta chama uma ferramenta e devolve o texto e se foi erro.
func ferramenta(t *testing.T, cs *sdk.ClientSession, nome string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: nome, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func jsonDe(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("resposta não é JSON: %v\n%s", err, s)
	}
	return v
}

// TestMCPCobraConsultaEDevolve faz pelo MCP o que um agente faria (#38): cria
// a cobrança, vê ela paga no sandbox, devolve parte e cancela outra.
func TestMCPCobraConsultaEDevolve(t *testing.T) {
	dir := t.TempDir()
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	url, token, parar := subirSandbox(t, filepath.Join(dir, "vintem-sandbox.db"), relogio)
	defer parar()
	cs := conectarMCP(t, mcp.API{URL: url, Token: token, ChavePix: "sandbox@vintem.dev"})

	lista, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var nomes []string
	for _, f := range lista.Tools {
		nomes = append(nomes, f.Name)
	}
	if got := strings.Join(nomes, ","); got != "cancelar_cobranca,consultar_cobranca,consultar_cobranca_vencimento,consultar_devolucao,criar_cobranca,criar_cobranca_vencimento,devolver_pix,listar_cobrancas" {
		t.Fatalf("ferramentas: %s", got)
	}

	texto, erro := ferramenta(t, cs, "criar_cobranca", map[string]any{"valor": "49.90", "descricao": "Pedido 42"})
	cob := jsonDe(t, texto)
	txid, _ := cob["txid"].(string)
	if erro || cob["status"] != "ATIVA" || txid == "" || cob["pixCopiaECola"] == "" {
		t.Fatalf("criar_cobranca: %s", texto)
	}

	relogio.Avancar(sandbox.AtrasoPadrao + time.Second)
	// O Pix pago entra em pix[] quando o Vintém o registra (webhook do banco ou
	// conciliação). O sandbox não chama webhook, e o serve concilia a cada 10 s
	// de relógio de verdade: aqui a conciliação roda na hora.
	conciliarSandbox(t, dir, relogio)
	texto, _ = ferramenta(t, cs, "consultar_cobranca", map[string]any{"txid": txid})
	paga := jsonDe(t, texto)
	pix, _ := paga["pix"].([]any)
	if paga["status"] != "CONCLUIDA" || len(pix) != 1 {
		t.Fatalf("consultar_cobranca depois do pagamento: %s", texto)
	}
	e2e, _ := pix[0].(map[string]any)["endToEndId"].(string)

	texto, erro = ferramenta(t, cs, "devolver_pix", map[string]any{"end_to_end_id": e2e, "valor": "10.00", "id": "dev1"})
	if erro || !strings.Contains(texto, `"dev1"`) {
		t.Fatalf("devolver_pix: %s", texto)
	}
	if texto, erro = ferramenta(t, cs, "devolver_pix", map[string]any{"end_to_end_id": e2e, "valor": "49.90", "id": "dev2"}); !erro {
		t.Fatalf("devolveu mais do que o recebido: %s", texto)
	}
	if texto, erro = ferramenta(t, cs, "consultar_devolucao", map[string]any{"end_to_end_id": e2e, "id": "dev1"}); erro || !strings.Contains(texto, `"10.00"`) {
		t.Fatalf("consultar_devolucao: %s", texto)
	}

	texto, _ = ferramenta(t, cs, "criar_cobranca", map[string]any{"valor": "15.00"})
	outra := jsonDe(t, texto)["txid"].(string)
	if texto, erro = ferramenta(t, cs, "cancelar_cobranca", map[string]any{"txid": outra}); erro || !strings.Contains(texto, "REMOVIDA_PELO_USUARIO_RECEBEDOR") {
		t.Fatalf("cancelar_cobranca: %s", texto)
	}

	texto, erro = ferramenta(t, cs, "listar_cobrancas", map[string]any{"inicio": "2026-10-29T00:00:00Z", "fim": "2026-10-30T00:00:00Z", "status": "CONCLUIDA"})
	if erro || !strings.Contains(texto, txid) || strings.Contains(texto, outra) {
		t.Fatalf("listar_cobrancas: %s", texto)
	}
}

// conciliarSandbox roda `vintem conciliar` no banco do sandbox, com o mesmo relógio.
func conciliarSandbox(t *testing.T, dir string, relogio *fake.Clock) {
	t.Helper()
	chave, err := os.ReadFile(filepath.Join(dir, "vintem-sandbox.key")) //nolint:gosec // G304: arquivo do próprio teste
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"VINTEM_DB": filepath.Join(dir, "vintem-sandbox.db"), "VINTEM_MASTER_KEY": strings.TrimSpace(string(chave))}
	amb := Ambiente{
		Saida: io.Discard, Getenv: func(k string) string { return env[k] }, Clock: relogio, Versao: "v-test",
		MontarPSPs: func(f Fontes) ports.Registry {
			return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store)))
		},
	}
	if err := Run(context.Background(), []string{"conciliar"}, amb); err != nil {
		t.Fatal(err)
	}
}

// TestMCPRespeitaOsEscoposDaAPIKey: com uma key só de leitura, o agente
// consulta, mas não cria cobrança nem devolve.
func TestMCPRespeitaOsEscoposDaAPIKey(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "vintem-sandbox.db")
	url, token, parar := subirSandbox(t, db, fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)))
	defer parar()

	chave, err := os.ReadFile(filepath.Join(dir, "vintem-sandbox.key")) //nolint:gosec // G304: arquivo do próprio teste
	if err != nil {
		t.Fatal(err)
	}
	amb, out := ambiente(t, map[string]string{"VINTEM_DB": db, "VINTEM_MASTER_KEY": strings.TrimSpace(string(chave))})
	if err := Run(context.Background(), []string{"apikey", "criar", "--nome", "agente", "--escopo", "cob:read"}, amb); err != nil {
		t.Fatal(err)
	}
	leitura := regexp.MustCompile(`vtm_\S+`).FindString(out.String())
	if leitura == "" {
		t.Fatalf("token não impresso: %s", out)
	}

	// Uma cobrança criada com a key completa, para a de leitura consultar.
	completo := conectarMCP(t, mcp.API{URL: url, Token: token, ChavePix: "sandbox@vintem.dev"})
	texto, _ := ferramenta(t, completo, "criar_cobranca", map[string]any{"valor": "20.00"})
	txid := jsonDe(t, texto)["txid"].(string)

	agente := conectarMCP(t, mcp.API{URL: url, Token: leitura, ChavePix: "sandbox@vintem.dev"})
	if texto, erro := ferramenta(t, agente, "consultar_cobranca", map[string]any{"txid": txid}); erro {
		t.Fatalf("leitura deveria passar: %s", texto)
	}
	for nome, args := range map[string]map[string]any{
		"criar_cobranca":    {"valor": "1.00"},
		"cancelar_cobranca": {"txid": txid},
		"devolver_pix":      {"end_to_end_id": "E12345678202610291200abcdefghijk", "valor": "1.00"},
	} {
		texto, erro := ferramenta(t, agente, nome, args)
		if !erro || !strings.Contains(texto, "403") || !strings.Contains(texto, "escopo") {
			t.Errorf("%s com key só de leitura: erro=%v %s", nome, erro, texto)
		}
	}
}

func TestMCPSemChaveNemTokenExplicaOQueFalta(t *testing.T) {
	cs := conectarMCP(t, mcp.API{URL: "http://127.0.0.1:1", Token: "x"})
	if texto, erro := ferramenta(t, cs, "criar_cobranca", map[string]any{"valor": "1.00"}); !erro || !strings.Contains(texto, "VINTEM_CHAVE_PIX") {
		t.Fatalf("sem chave: %v %s", erro, texto)
	}
	amb, _ := ambiente(t, map[string]string{})
	if err := Run(context.Background(), []string{"mcp"}, amb); err == nil || !strings.Contains(err.Error(), "VINTEM_TOKEN") {
		t.Fatalf("sem token: %v", err)
	}
}

// TestMCPListarCobrancasSemPeriodoUsaAsUltimas24Horas (#119): sem inicio e fim,
// o agente vê o que entrou nas últimas 24 h; com só um dos dois, a ferramenta
// explica o que falta sem chamar a API.
func TestMCPListarCobrancasSemPeriodoUsaAsUltimas24Horas(t *testing.T) {
	dir := t.TempDir()
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	url, token, parar := subirSandbox(t, filepath.Join(dir, "vintem-sandbox.db"), relogio)
	defer parar()
	cs := conectarMCP(t, mcp.API{URL: url, Token: token, ChavePix: "sandbox@vintem.dev", Agora: relogio.Now})

	texto, _ := ferramenta(t, cs, "criar_cobranca", map[string]any{"valor": "30.00"})
	antiga := jsonDe(t, texto)["txid"].(string)
	relogio.Avancar(25 * time.Hour)
	texto, _ = ferramenta(t, cs, "criar_cobranca", map[string]any{"valor": "31.00"})
	nova := jsonDe(t, texto)["txid"].(string)
	relogio.Avancar(time.Minute) // o relógio falso não anda sozinho

	texto, erro := ferramenta(t, cs, "listar_cobrancas", map[string]any{})
	if erro || !strings.Contains(texto, nova) || strings.Contains(texto, antiga) {
		t.Fatalf("listar_cobrancas sem período: %s", texto)
	}
	for _, args := range []map[string]any{{"inicio": "2026-10-29T00:00:00Z"}, {"fim": "2026-10-31T00:00:00Z"}} {
		texto, erro := ferramenta(t, cs, "listar_cobrancas", args)
		if !erro || !strings.Contains(texto, "inicio e fim juntos") || strings.Contains(texto, "o Vintém respondeu") {
			t.Errorf("%v: erro=%v %s", args, erro, texto)
		}
	}
}

// TestMCPCobrancaComVencimento (#114): o agente emite o "boleto do Pix" e o
// consulta pelo txid, gerado quando ele não informa um.
func TestMCPCobrancaComVencimento(t *testing.T) {
	dir := t.TempDir()
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	url, token, parar := subirSandbox(t, filepath.Join(dir, "vintem-sandbox.db"), relogio)
	defer parar()
	cs := conectarMCP(t, mcp.API{URL: url, Token: token, ChavePix: "sandbox@vintem.dev"})

	texto, erro := ferramenta(t, cs, "criar_cobranca_vencimento", map[string]any{
		"valor": "120.00", "vencimento": "2026-11-30", "devedor_nome": "Fulano de Tal", "devedor_documento": "12345678909",
		"validade_apos_vencimento": 10, "multa_percentual": "2.00", "juros_percentual_mes": "1.00", "descricao": "Mensalidade de novembro",
	})
	cobv := jsonDe(t, texto)
	txid, _ := cobv["txid"].(string)
	if erro || cobv["status"] != "ATIVA" || len(txid) < 26 || cobv["pixCopiaECola"] == "" {
		t.Fatalf("criar_cobranca_vencimento: %s", texto)
	}
	calendario, _ := cobv["calendario"].(map[string]any)
	valor, _ := cobv["valor"].(map[string]any)
	if calendario["dataDeVencimento"] != "2026-11-30" || calendario["validadeAposVencimento"] != float64(10) || valor["multa"] == nil || valor["juros"] == nil {
		t.Fatalf("campos da cobv: %s", texto)
	}

	texto, erro = ferramenta(t, cs, "consultar_cobranca_vencimento", map[string]any{"txid": txid})
	if erro || jsonDe(t, texto)["txid"] != txid || !strings.Contains(texto, "Fulano de Tal") {
		t.Fatalf("consultar_cobranca_vencimento: %s", texto)
	}

	// Com o txid informado, tentar de novo devolve a mesma cobrança.
	meu := "pedido42vencimentonovembro0001"
	args := map[string]any{"valor": "50.00", "vencimento": "2026-11-30", "devedor_nome": "Empresa", "devedor_documento": "11222333000181", "txid": meu}
	if texto, erro = ferramenta(t, cs, "criar_cobranca_vencimento", args); erro || jsonDe(t, texto)["txid"] != meu || !strings.Contains(texto, `"cnpj"`) {
		t.Fatalf("com txid: %s", texto)
	}
	if texto, erro = ferramenta(t, cs, "criar_cobranca_vencimento", args); erro || jsonDe(t, texto)["txid"] != meu {
		t.Fatalf("tentar de novo com o mesmo txid: %s", texto)
	}

	if texto, erro = ferramenta(t, cs, "criar_cobranca_vencimento", map[string]any{"valor": "1.00", "vencimento": "2026-11-30"}); !erro || !strings.Contains(texto, "devedor") || strings.Contains(texto, "o Vintém respondeu") {
		t.Fatalf("sem devedor: %v %s", erro, texto)
	}
}
