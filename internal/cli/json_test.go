package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rodarJSON roda o comando com --json e decodifica o array impresso.
func rodarJSON[T any](t *testing.T, amb Ambiente, saida interface {
	String() string
	Reset()
}, args ...string) []T {
	t.Helper()
	saida.Reset()
	if err := Run(context.Background(), append(args, "--json"), amb); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	var itens []T
	if err := json.Unmarshal([]byte(saida.String()), &itens); err != nil {
		t.Fatalf("%v: não é um array JSON: %v\n%s", args, err, saida)
	}
	return itens
}

func TestContaListarJSON(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	ctx := context.Background()
	if got := rodarJSON[contaJSON](t, amb, out, "conta", "listar"); len(got) != 0 {
		t.Fatalf("sem contas: %v", got)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "[") {
		t.Fatalf("lista vazia não saiu como array: %q", out)
	}
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("segredo-do-arquivo"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox",
		"--chave", "x@example.com", "--client-id", "abc", "--client-secret-arquivo", secret}, amb); err != nil {
		t.Fatal(err)
	}
	got := rodarJSON[contaJSON](t, amb, out, "conta", "listar")
	if len(got) != 1 || got[0].ID != "loja" || got[0].PSP != "sandbox" || got[0].ChavePix != "x@example.com" || !got[0].Ativa || got[0].CertificadoValidade != nil {
		t.Fatalf("conta: %+v", got)
	}
	if strings.Contains(out.String(), "segredo-do-arquivo") || strings.Contains(out.String(), "abc") {
		t.Fatalf("o JSON mostrou credenciais: %s", out)
	}

	// Sem --json, a tabela continua igual.
	out.Reset()
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil || !strings.HasPrefix(out.String(), "ID") {
		t.Fatalf("tabela: %v\n%s", err, out)
	}
	if err := Run(ctx, []string{"conta", "listar", "--yaml"}, amb); err == nil {
		t.Fatal("aceitou flag desconhecida")
	}
}

func TestAPIKeyListarJSON(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	if err := Run(context.Background(), []string{"apikey", "criar", "--nome", "loja", "--escopo", "cob:read"}, amb); err != nil {
		t.Fatal(err)
	}
	var token string
	for _, campo := range strings.Fields(out.String()) {
		if strings.HasPrefix(campo, "vtm_") {
			token = campo
		}
	}
	got := rodarJSON[apikeyJSON](t, amb, out, "apikey", "listar")
	if len(got) != 1 || got[0].Nome != "loja" || len(got[0].Escopos) != 1 || got[0].Escopos[0] != "cob:read" || got[0].RevogadaEm != nil || got[0].CriadaEm.IsZero() {
		t.Fatalf("api key: %+v", got)
	}
	if token == "" || strings.Contains(out.String(), token) || strings.Contains(out.String(), "vtm_") {
		t.Fatalf("o JSON mostrou o token: %s", out)
	}
	if err := Run(context.Background(), []string{"apikey", "revogar", got[0].ID}, amb); err != nil {
		t.Fatal(err)
	}
	if got = rodarJSON[apikeyJSON](t, amb, out, "apikey", "listar"); got[0].RevogadaEm == nil {
		t.Fatalf("revogada sem data: %+v", got)
	}
}

func TestDestinoEDLQListarJSON(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	if err := Run(context.Background(), []string{"destino", "criar", "--url", "https://app.example.com/webhooks/vintem"}, amb); err != nil {
		t.Fatal(err)
	}
	got := rodarJSON[destinoJSON](t, amb, out, "destino", "listar")
	if len(got) != 1 || got[0].URL != "https://app.example.com/webhooks/vintem" || got[0].ContaID != "" || got[0].CriadoEm.IsZero() {
		t.Fatalf("destino: %+v", got)
	}
	if strings.Contains(out.String(), "whsec_") || strings.Contains(strings.ToLower(out.String()), "segredo") {
		t.Fatalf("o JSON mostrou o segredo: %s", out)
	}
	if dlq := rodarJSON[dlqJSON](t, amb, out, "dlq", "listar"); len(dlq) != 0 {
		t.Fatalf("dlq: %+v", dlq)
	}
}
