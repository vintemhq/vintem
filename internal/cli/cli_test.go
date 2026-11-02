package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/platform/vault"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

type pspFalso struct{ ports.PSP }

func (pspFalso) Nome() string { return "sandbox" }

func ambiente(t *testing.T, env map[string]string) (Ambiente, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return Ambiente{Saida: &out, Getenv: func(k string) string { return env[k] }, PSPs: psp.NewRegistry(pspFalso{}), Clock: fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)), Versao: "v-test"}, &out
}

func envComBanco(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"VINTEM_DB": filepath.Join(t.TempDir(), "vintem.db"), "VINTEM_MASTER_KEY": vault.NovaChave()}
}

func TestVersionEChave(t *testing.T) {
	amb, out := ambiente(t, nil)
	if err := Run(context.Background(), []string{"version"}, amb); err != nil || strings.TrimSpace(out.String()) != "v-test" {
		t.Fatalf("version: %q, %v", out, err)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"chave", "gerar"}, amb); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.New(strings.TrimSpace(out.String())); err != nil {
		t.Fatalf("chave gerada é inválida: %v", err)
	}
}

func TestContaSemMasterKeyFalhaComOrientacao(t *testing.T) {
	amb, _ := ambiente(t, map[string]string{"VINTEM_DB": filepath.Join(t.TempDir(), "x.db")})
	err := Run(context.Background(), []string{"conta", "listar"}, amb)
	if err == nil || !strings.Contains(err.Error(), "vintem chave gerar") {
		t.Fatalf("err = %v", err)
	}
}

func TestContaCriarListarDesativar(t *testing.T) {
	env := envComBanco(t)
	amb, out := ambiente(t, env)
	ctx := context.Background()

	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("segredo-do-arquivo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox",
		"--chave", "x@example.com", "--client-id", "abc", "--client-secret-arquivo", secret}, amb)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "segredo-do-arquivo") {
		t.Fatal("a saída mostrou o segredo")
	}

	out.Reset()
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "loja") || !strings.Contains(out.String(), "true") || strings.Contains(out.String(), "segredo") {
		t.Fatalf("listar: %s", out)
	}

	out.Reset()
	if err := Run(ctx, []string{"conta", "desativar", "loja"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	_ = Run(ctx, []string{"conta", "listar"}, amb)
	if !strings.Contains(out.String(), "false") {
		t.Fatalf("depois de desativar: %s", out)
	}

	// O segredo foi gravado sem a quebra de linha do arquivo, e só sai decifrado.
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	cred, err := contas.Credenciais(ctx, "loja")
	if err != nil || cred.ClientSecret != "segredo-do-arquivo" || cred.ClientID != "abc" {
		t.Fatalf("credenciais: id=%q, %v", cred.ClientID, err)
	}
}

func TestContaCriarComPSPDesconhecido(t *testing.T) {
	amb, _ := ambiente(t, envComBanco(t))
	err := Run(context.Background(), []string{"conta", "criar", "--id", "x", "--nome", "X", "--psp", "itau", "--chave", "k"}, amb)
	if err == nil || !strings.Contains(err.Error(), "disponíveis") {
		t.Fatalf("err = %v", err)
	}
}

func TestComandoDesconhecido(t *testing.T) {
	amb, _ := ambiente(t, nil)
	if err := Run(context.Background(), []string{"voar"}, amb); err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("err = %v", err)
	}
}

func TestAPIKeyCriarListarRevogar(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	ctx := context.Background()
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "loja", "--escopo", "cob:read", "--escopo", "cob:write"}, amb); err != nil {
		t.Fatal(err)
	}
	var token, id string
	for _, campo := range strings.Fields(out.String()) {
		if strings.HasPrefix(campo, "vtm_") {
			token = campo
		}
	}
	id = strings.Fields(out.String())[2]
	if token == "" {
		t.Fatalf("token não apareceu: %s", out)
	}
	out.Reset()
	_ = Run(ctx, []string{"apikey", "listar"}, amb)
	if !strings.Contains(out.String(), id) || strings.Contains(out.String(), token) {
		t.Fatalf("listar: %s", out)
	}
	if err := Run(ctx, []string{"apikey", "revogar", id}, amb); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "x", "--escopo", "tudo"}, amb); err == nil {
		t.Fatal("aceitou escopo inventado")
	}
}

func TestContaWebhookValidaArgumentos(t *testing.T) {
	amb, _ := ambiente(t, envComBanco(t))
	ctx := context.Background()
	if err := Run(ctx, []string{"conta", "webhook"}, amb); err == nil || !strings.Contains(err.Error(), "use:") {
		t.Fatalf("sem id: err = %v", err)
	}
	if err := Run(ctx, []string{"conta", "webhook", "loja"}, amb); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("sem --url-publica: err = %v", err)
	}
}

func TestConciliarEValidaDuracoes(t *testing.T) {
	env := envComBanco(t)
	amb, out := ambiente(t, env)
	if err := Run(context.Background(), []string{"conciliar"}, amb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0 contas conciliadas") {
		t.Fatalf("saída: %s", out)
	}
	env["VINTEM_CONCILIACAO_JANELA"] = "duas horas"
	if err := Run(context.Background(), []string{"conciliar"}, amb); err == nil || !strings.Contains(err.Error(), "VINTEM_CONCILIACAO_JANELA") {
		t.Fatalf("janela inválida: err = %v", err)
	}
	env["VINTEM_CONCILIACAO_JANELA"] = "0s"
	if err := Run(context.Background(), []string{"conciliar"}, amb); err == nil {
		t.Fatal("aceitou janela zero")
	}
}

func TestDestinoEDLQ(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	ctx := context.Background()
	if err := Run(ctx, []string{"destino", "criar", "--url", "https://app.example.com/webhooks/vintem"}, amb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "whsec_") {
		t.Fatalf("o segredo não apareceu: %s", out)
	}
	id := strings.Fields(out.String())[1]
	out.Reset()
	_ = Run(ctx, []string{"destino", "listar"}, amb)
	if !strings.Contains(out.String(), id) || !strings.Contains(out.String(), "(todas)") || strings.Contains(out.String(), "whsec_") {
		t.Fatalf("listar: %s", out)
	}
	if err := Run(ctx, []string{"destino", "criar", "--url", "http://app.example.com"}, amb); err == nil {
		t.Fatal("aceitou http fora de localhost")
	}
	if err := Run(ctx, []string{"dlq", "reenviar", "evt_inexistente"}, amb); err == nil || !strings.Contains(err.Error(), "DLQ") {
		t.Fatalf("reenviar sem nada na DLQ: err = %v", err)
	}
	if err := Run(ctx, []string{"destino", "remover", id}, amb); err != nil {
		t.Fatal(err)
	}
}

// Os adapters de banco real recebem as credenciais pelo MontarPSPs, montado
// depois de abrir o store e o vault.
func TestMontarPSPsRecebeAsCredenciaisDasContas(t *testing.T) {
	amb, _ := ambiente(t, envComBanco(t))
	var recebido ports.Credenciais
	amb.MontarPSPs = func(f Fontes) ports.Registry {
		recebido = f.Credenciais
		return psp.NewRegistry(pspFalso{})
	}
	ctx := context.Background()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("secret-do-banco"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "x@example.com",
		"--client-id", "abc", "--client-secret-arquivo", secret}, amb); err != nil {
		t.Fatal(err)
	}
	// O provedor vale enquanto o store estiver aberto, como no serve.
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	if recebido == nil || contas.PSPs == nil {
		t.Fatal("MontarPSPs não foi chamado")
	}
	cred, err := recebido.Credenciais(ctx, "loja")
	if err != nil || cred.ClientSecret != "secret-do-banco" || cred.ClientID != "abc" {
		t.Fatalf("Credenciais = %q, %v", cred.ClientID, err)
	}
}

// -h mostra a ajuda do comando sem rodá-lo: o sandbox não sobe e o conta
// criar não pede a VINTEM_MASTER_KEY.
func TestAjudaDosSubcomandosNaoRodaNada(t *testing.T) {
	db := filepath.Join(t.TempDir(), "x.db")
	casos := map[string][]string{
		"sandbox":         {"sandbox", "-h"},
		"conta criar":     {"conta", "criar", "--help"},
		"apikey criar":    {"apikey", "criar", "-h"},
		"conta webhook":   {"conta", "webhook", "-h"},
		"destino remover": {"destino", "remover", "-h"},
	}
	for quer, args := range casos {
		amb, out := ambiente(t, map[string]string{"VINTEM_DB": db})
		if err := Run(context.Background(), args, amb); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out.String(), "  "+quer) {
			t.Errorf("%v: ajuda sem %q:\n%s", args, quer, out)
		}
	}
	amb, out := ambiente(t, map[string]string{"VINTEM_DB": db})
	if err := Run(context.Background(), []string{"conta", "criar", "-h"}, amb); err != nil || !strings.Contains(out.String(), "-chave-aplicacao") {
		t.Fatalf("flags do conta criar: %v\n%s", err, out)
	}
	if _, err := os.Stat(db); !os.IsNotExist(err) {
		t.Fatalf("a ajuda abriu o banco: %v", err)
	}
}

func TestChaveTrocarRecifraTudo(t *testing.T) {
	ctx := context.Background()
	env := envComBanco(t)
	amb, out := ambiente(t, env)
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@exemplo.com"}, amb); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"destino", "criar", "--url", "https://exemplo.com/h"}, amb); err != nil {
		t.Fatal(err)
	}

	env["VINTEM_MASTER_KEY_NOVA"] = env["VINTEM_MASTER_KEY"]
	if err := Run(ctx, []string{"chave", "trocar"}, amb); err == nil || !strings.Contains(err.Error(), "igual") {
		t.Fatalf("chave nova igual à atual: %v", err)
	}
	env["VINTEM_MASTER_KEY_NOVA"] = "curta"
	if err := Run(ctx, []string{"chave", "trocar"}, amb); err == nil || !strings.Contains(err.Error(), "VINTEM_MASTER_KEY_NOVA") {
		t.Fatalf("chave nova inválida: %v", err)
	}
	delete(env, "VINTEM_MASTER_KEY_NOVA")
	if err := Run(ctx, []string{"chave", "trocar"}, amb); err == nil || !strings.Contains(err.Error(), "defina") {
		t.Fatalf("sem chave nova: %v", err)
	}

	antiga, nova := env["VINTEM_MASTER_KEY"], vault.NovaChave()
	env["VINTEM_MASTER_KEY_NOVA"] = nova
	out.Reset()
	if err := Run(ctx, []string{"chave", "trocar"}, amb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1 contas e 1 destinos") {
		t.Fatalf("saída: %s", out)
	}

	// A antiga deixa de abrir o banco, e rodar de novo não estraga nada.
	if err := Run(ctx, []string{"conta", "listar"}, amb); err == nil || !strings.Contains(err.Error(), "não abre os dados") {
		t.Fatalf("chave antiga depois da troca: %v", err)
	}
	if err := Run(ctx, []string{"chave", "trocar"}, amb); err == nil || !strings.Contains(err.Error(), "não abre os dados") {
		t.Fatalf("trocar de novo com a antiga: %v", err)
	}
	env["VINTEM_MASTER_KEY"], env["VINTEM_MASTER_KEY_NOVA"] = nova, antiga
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
		t.Fatalf("chave nova: %v", err)
	}
}

func TestChaveMestraDiferenteRecusadaAoAbrir(t *testing.T) {
	ctx := context.Background()
	env := envComBanco(t)
	amb, _ := ambiente(t, env)
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@exemplo.com"}, amb); err != nil {
		t.Fatal(err)
	}
	env["VINTEM_MASTER_KEY"] = vault.NovaChave()
	for _, comando := range [][]string{{"conta", "listar"}, {"serve"}} {
		if err := Run(ctx, comando, amb); err == nil || !strings.Contains(err.Error(), "não abre os dados") {
			t.Fatalf("%v: %v", comando, err)
		}
	}
}

func TestBancoDoLastroNaoViraBancoVazio(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("lastro.db", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	env := envComBanco(t)
	delete(env, "VINTEM_DB")
	amb, _ := ambiente(t, env)
	ctx := context.Background()
	if err := Run(ctx, []string{"conta", "listar"}, amb); err == nil || !strings.Contains(err.Error(), "lastro.db") {
		t.Fatalf("banco do Lastro esquecido: %v", err)
	}
	if _, err := os.Stat("vintem.db"); err == nil {
		t.Fatal("criou um vintem.db vazio")
	}
	env["VINTEM_DB"] = "lastro.db"
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
		t.Fatalf("com VINTEM_DB: %v", err)
	}
	delete(env, "VINTEM_DB")
	if err := os.Rename("lastro.db", "vintem.db"); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
		t.Fatalf("depois de renomear: %v", err)
	}
}

// TestContaWebhookRemover (#178): registra e cancela pela CLI, contra o sandbox.
func TestContaWebhookRemover(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	amb.PSPs = psp.NewRegistry(sandbox.New(fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))))
	ctx := context.Background()
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@exemplo.com"}, amb); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--url-publica", "https://vintem.example.com", "--remover"}, amb); err == nil || !strings.Contains(err.Error(), "não os dois") {
		t.Fatalf("as duas flags: %v", err)
	}
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--url-publica", "https://vintem.example.com"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--remover"}, amb); err != nil || !strings.Contains(out.String(), "cancelado no PSP") {
		t.Fatalf("remover: %v\n%s", err, out)
	}
	out.Reset()
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--remover"}, amb); err != nil || !strings.Contains(out.String(), "não tinha webhook") {
		t.Fatalf("remover de novo: %v\n%s", err, out)
	}
}
