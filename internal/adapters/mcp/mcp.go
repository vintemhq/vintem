package mcp

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// API é a API HTTP do Vintém que as ferramentas chamam, com a API key do
// agente. Os escopos da key valem aqui como valem para qualquer cliente
// (ADR-008): ferramenta de escrita com key só de leitura recebe o 403.
type API struct {
	URL   string // ex.: http://localhost:8080
	Token string // API key vtm_...
	// ChavePix é a chave usada quando o agente não informa outra.
	ChavePix string
	HTTP     *http.Client
	// Agora dá a hora usada no período padrão de listar_cobrancas; nil usa time.Now.
	Agora func() time.Time
}

// periodoPadrao é o que listar_cobrancas lista quando o agente não dá o período.
const periodoPadrao = 24 * time.Hour

// NovoServidor monta o servidor MCP com as ferramentas de cobrança e devolução.
func NovoServidor(api API, versao string) *sdk.Server {
	if api.HTTP == nil {
		api.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if api.Agora == nil {
		api.Agora = time.Now
	}
	api.URL = strings.TrimRight(api.URL, "/")
	s := sdk.NewServer(&sdk.Implementation{Name: "vintem", Title: "Vintém: Pix sem gateway", Version: versao}, &sdk.ServerOptions{
		Instructions: "Ferramentas para cobrar por Pix pelo Vintém, um gateway Pix self-hosted. " +
			"Valores são em reais com duas casas (\"49.90\"). Uma cobrança nasce ATIVA e vira CONCLUIDA quando o pagador paga; " +
			"consultar uma cobrança ATIVA pergunta ao banco se ela já foi paga. Mostre o pixCopiaECola para o pagador copiar.",
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "criar_cobranca",
		Description: "Cria uma cobrança Pix imediata (cob) e devolve o txid e o Pix Copia e Cola para o pagador.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in CriarCobranca) (*sdk.CallToolResult, any, error) {
		corpo := map[string]any{
			"calendario": map[string]any{"expiracao": cmp.Or(in.ExpiracaoSegundos, 3600)},
			"valor":      map[string]any{"original": in.Valor},
		}
		chave := cmp.Or(in.Chave, api.ChavePix)
		if chave == "" {
			return erroParaAgente("informe a chave Pix que recebe (ou defina VINTEM_CHAVE_PIX no vintem mcp)"), nil, nil
		}
		corpo["chave"] = chave
		if in.Descricao != "" {
			corpo["solicitacaoPagador"] = in.Descricao
		}
		if in.DevedorDocumento != "" {
			devedor := map[string]string{"nome": in.DevedorNome}
			if len(in.DevedorDocumento) == 14 {
				devedor["cnpj"] = in.DevedorDocumento
			} else {
				devedor["cpf"] = in.DevedorDocumento
			}
			corpo["devedor"] = devedor
		}
		return api.chamar(ctx, http.MethodPost, "/cob", corpo, map[string]string{"Idempotency-Key": cmp.Or(in.ChaveIdempotencia, idAleatorio())})
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "consultar_cobranca",
		Description: "Consulta uma cobrança Pix imediata pelo txid. Se ela ainda estiver ATIVA, o Vintém pergunta ao banco se já foi paga. " +
			"Os pagamentos aparecem em pix[], com o endToEndId que a devolução pede, quando o Vintém registra o Pix (pelo webhook do banco ou pela conciliação, que roda sozinha).",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in Txid) (*sdk.CallToolResult, any, error) {
		return api.chamar(ctx, http.MethodGet, "/cob/"+url.PathEscape(in.Txid), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name: "listar_cobrancas",
		Description: "Lista as cobranças Pix imediatas criadas num período, opcionalmente por status. " +
			"Sem inicio e fim, lista as das últimas 24 horas; para outro período, informe os dois.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ListarCobrancas) (*sdk.CallToolResult, any, error) {
		inicio, fim := in.Inicio, in.Fim
		switch {
		case inicio == "" && fim == "":
			agora := api.Agora()
			inicio, fim = agora.Add(-periodoPadrao).Format(time.RFC3339), agora.Format(time.RFC3339)
		case inicio == "":
			return erroParaAgente("falta inicio: informe inicio e fim juntos (RFC 3339), ou nenhum dos dois para as últimas 24 horas"), nil, nil
		case fim == "":
			return erroParaAgente("falta fim: informe inicio e fim juntos (RFC 3339), ou nenhum dos dois para as últimas 24 horas"), nil, nil
		}
		q := url.Values{"inicio": {inicio}, "fim": {fim}}
		if in.Status != "" {
			q.Set("status", in.Status)
		}
		return api.chamar(ctx, http.MethodGet, "/cob?"+q.Encode(), nil, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "cancelar_cobranca",
		Description: "Cancela uma cobrança Pix imediata que ainda não foi paga (status REMOVIDA_PELO_USUARIO_RECEBEDOR).",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in Txid) (*sdk.CallToolResult, any, error) {
		return api.chamar(ctx, http.MethodPatch, "/cob/"+url.PathEscape(in.Txid), map[string]string{"status": "REMOVIDA_PELO_USUARIO_RECEBEDOR"}, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "devolver_pix",
		Description: "Devolve um Pix recebido, total ou parcialmente. O Vintém recusa devolver mais do que foi recebido.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in Devolver) (*sdk.CallToolResult, any, error) {
		corpo := map[string]any{"valor": in.Valor}
		if in.Descricao != "" {
			corpo["descricao"] = in.Descricao
		}
		return api.chamar(ctx, http.MethodPut, "/pix/"+url.PathEscape(in.EndToEndID)+"/devolucao/"+url.PathEscape(cmp.Or(in.ID, idAleatorio())), corpo, nil)
	})

	sdk.AddTool(s, &sdk.Tool{
		Name:        "consultar_devolucao",
		Description: "Consulta uma devolução pelo endToEndId do Pix e pelo id da devolução.",
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in ConsultarDevolucao) (*sdk.CallToolResult, any, error) {
		return api.chamar(ctx, http.MethodGet, "/pix/"+url.PathEscape(in.EndToEndID)+"/devolucao/"+url.PathEscape(in.ID), nil, nil)
	})
	return s
}

// Entradas das ferramentas. As descrições viram o JSON Schema que o agente lê.
type (
	// CriarCobranca é a entrada de criar_cobranca.
	CriarCobranca struct {
		Valor             string `json:"valor" jsonschema:"valor em reais com duas casas, ex. 49.90"`
		Descricao         string `json:"descricao,omitempty" jsonschema:"texto que o pagador vê no app do banco (até 140 caracteres)"`
		ExpiracaoSegundos int    `json:"expiracao_segundos,omitempty" jsonschema:"segundos até a cobrança expirar; padrão 3600"`
		Chave             string `json:"chave,omitempty" jsonschema:"chave Pix que recebe; vazia usa VINTEM_CHAVE_PIX do servidor MCP"`
		DevedorNome       string `json:"devedor_nome,omitempty" jsonschema:"nome de quem paga, se quiser identificar"`
		DevedorDocumento  string `json:"devedor_documento,omitempty" jsonschema:"CPF (11 dígitos) ou CNPJ (14 dígitos) de quem paga, só números"`
		ChaveIdempotencia string `json:"chave_idempotencia,omitempty" jsonschema:"repita o mesmo valor para não criar a cobrança duas vezes ao tentar de novo"`
	}
	// Txid é a entrada de consultar_cobranca e cancelar_cobranca.
	Txid struct {
		Txid string `json:"txid" jsonschema:"txid da cobrança"`
	}
	// ListarCobrancas é a entrada de listar_cobrancas.
	ListarCobrancas struct {
		Inicio string `json:"inicio,omitempty" jsonschema:"início do período, RFC 3339 (ex. 2026-10-29T00:00:00-03:00); sem inicio e fim, lista as últimas 24 horas"`
		Fim    string `json:"fim,omitempty" jsonschema:"fim do período, RFC 3339; informe junto com inicio"`
		Status string `json:"status,omitempty" jsonschema:"filtra por status: ATIVA, CONCLUIDA, REMOVIDA_PELO_USUARIO_RECEBEDOR ou REMOVIDA_PELO_PSP"`
	}
	// Devolver é a entrada de devolver_pix.
	Devolver struct {
		EndToEndID string `json:"end_to_end_id" jsonschema:"endToEndId do Pix recebido (32 caracteres, começa com E)"`
		Valor      string `json:"valor" jsonschema:"valor a devolver em reais, ex. 10.00"`
		ID         string `json:"id,omitempty" jsonschema:"id da devolução; repita o mesmo para não devolver duas vezes. Vazio gera um novo"`
		Descricao  string `json:"descricao,omitempty" jsonschema:"motivo que o pagador vê"`
	}
	// ConsultarDevolucao é a entrada de consultar_devolucao.
	ConsultarDevolucao struct {
		EndToEndID string `json:"end_to_end_id" jsonschema:"endToEndId do Pix"`
		ID         string `json:"id" jsonschema:"id da devolução"`
	}
)

// chamar faz a requisição e devolve o corpo como resultado da ferramenta. Erro
// da API (4xx/5xx) vira resultado com IsError, com o corpo do problema, para o
// agente ler o motivo; falha de rede vira erro.
func (api API) chamar(ctx context.Context, metodo, caminho string, corpo any, headers map[string]string) (*sdk.CallToolResult, any, error) {
	var leitor io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return nil, nil, err
		}
		leitor = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, api.URL+caminho, leitor)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+api.Token)
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := api.HTTP.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("o Vintém em %s não respondeu: %w", api.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, nil, err
	}
	texto := string(b)
	if resp.StatusCode >= 400 {
		texto = fmt.Sprintf("o Vintém respondeu %d: %s", resp.StatusCode, b)
		if resp.StatusCode == http.StatusForbidden {
			texto += "\n(a API key do agente não tem o escopo para esta ferramenta)"
		}
	}
	return &sdk.CallToolResult{
		IsError: resp.StatusCode >= 400,
		Content: []sdk.Content{&sdk.TextContent{Text: texto}},
	}, nil, nil
}

func erroParaAgente(texto string) *sdk.CallToolResult {
	return &sdk.CallToolResult{IsError: true, Content: []sdk.Content{&sdk.TextContent{Text: texto}}}
}

// ErrSemToken indica que o agente não recebeu uma API key.
var ErrSemToken = errors.New("defina VINTEM_TOKEN com uma API key do Vintém (vintem apikey criar)")

// idAleatorio gera um identificador de 32 caracteres para idempotência e devolução.
func idAleatorio() string {
	const alfabeto = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alfabeto[int(b[i])%len(alfabeto)]
	}
	return string(b)
}
