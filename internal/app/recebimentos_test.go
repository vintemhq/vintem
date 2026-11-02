package app_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

func pixDe(e2e, txid string, valor domain.Money) domain.PixRecebido {
	return domain.PixRecebido{EndToEndID: e2e, ContaID: "loja", Txid: domain.Txid(txid), Valor: valor, Horario: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
}

func eventosDoTipo(evs []domain.Evento, tipo domain.TipoEvento) []domain.Evento {
	var out []domain.Evento
	for _, e := range evs {
		if e.Tipo == tipo {
			out = append(out, e)
		}
	}
	return out
}

func TestRecebimentoConcluiACobUmaVezSo(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}

	p := pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor)
	for range 2 { // o mesmo Pix avisado duas vezes
		if _, err := rec.Registrar(ctx, []domain.PixRecebido{p}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid)
	if got.Status != domain.CobrancaConcluida || got.Revisao != 1 {
		t.Fatalf("cob: %s rev %d", got.Status, got.Revisao)
	}
	evs := c.store.Eventos()
	if n := len(eventosDoTipo(evs, domain.EventoPixRecebido)); n != 1 {
		t.Fatalf("%d eventos pix.recebido, quer 1", n)
	}
	if n := len(eventosDoTipo(evs, domain.EventoCobrancaConcluida)); n != 1 {
		t.Fatalf("%d eventos cobranca.concluida, quer 1", n)
	}
	pixes, _ := rec.DaCobranca(ctx, "loja", cob.Txid)
	if len(pixes) != 1 {
		t.Fatalf("DaCobranca = %d", len(pixes))
	}
}

func TestRecebimentoComValorDivergente(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	_, _ = rec.Registrar(ctx, []domain.PixRecebido{pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor-1)})
	ev := eventosDoTipo(c.store.Eventos(), domain.EventoPixRecebido)[0]
	if !ev.Dados.(app.DadosPix).Divergente {
		t.Fatal("valor diferente não foi marcado como divergente")
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("o banco liquidou: a cob deveria concluir mesmo divergente, está %s", got.Status)
	}
}

func TestRecebimentoNaoConcluiCobDeOutraContaNemSemTxid(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	alheio := pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor)
	alheio.ContaID = "fechada"
	semTxid := pixDe("E12345678202610291201abcdefghijk", "", 500)
	if n, err := rec.Registrar(ctx, []domain.PixRecebido{alheio, semTxid}); err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaAtiva {
		t.Fatalf("Pix de outra conta concluiu a cob: %s", got.Status)
	}
}

func TestRecebimentoConcluiCobv(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	v, _ := c.svc.Criar(ctx, pedidoV())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	_, _ = rec.Registrar(ctx, []domain.PixRecebido{pixDe("E12345678202610291200abcdefghijk", string(v.Txid), 14000)})
	if got, _ := c.store.CobrancasV().BuscarPorTxid(ctx, v.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("cobv: %s", got.Status)
	}
}

func TestWebhookConfigurarEReceber(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	reg := psp.NewRegistry(sandbox.New(c.clock))
	wh := app.Webhooks{Store: c.store, PSPs: reg, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}}

	if _, err := wh.Configurar(ctx, "loja", "http://vintem.example.com"); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("http fora de localhost: err = %v", err)
	}
	endereco, err := wh.Configurar(ctx, "loja", "https://vintem.example.com/")
	if err != nil || !strings.HasPrefix(endereco, "https://vintem.example.com/webhooks/pix/loja/") {
		t.Fatalf("Configurar = %q, %v", endereco, err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]

	cob, _ := c.svc.Criar(ctx, pedido())
	corpo := `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","txid":"` + string(cob.Txid) + `","valor":"10.50","horario":"2026-10-29T12:00:00Z"}]}`
	if _, err := wh.Receber(ctx, "loja", "token-errado", []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token errado: err = %v", err)
	}
	if _, err := wh.Receber(ctx, "loja", token, []byte(`{"pix":[]}`)); !errors.Is(err, domain.ErrPixInvalido) {
		t.Fatalf("corpo inválido: err = %v", err)
	}
	if n, err := wh.Receber(ctx, "loja", token, []byte(corpo)); err != nil || n != 1 {
		t.Fatalf("Receber: n=%d err=%v", n, err)
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("cob: %s", got.Status)
	}
	if _, err := wh.Receber(ctx, "fechada", token, []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token de outra conta: err = %v", err)
	}
	// Reconfigurar troca o token: o antigo para de valer.
	_, _ = wh.Configurar(ctx, "loja", "https://vintem.example.com")
	if _, err := wh.Receber(ctx, "loja", token, []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatal("token antigo continuou valendo")
	}
}

// bancoQueChamaAURL faz como o Efí: chama a URL durante o registro e só aceita se
// o token já valer. Com recusar, o banco recusa depois da chamada.
type bancoQueChamaAURL struct {
	*sandbox.PSP
	wh      *app.Webhooks
	recusar error
}

func (b *bancoQueChamaAURL) ConfigurarWebhook(ctx context.Context, conta domain.Conta, url string) error {
	token := url[strings.LastIndex(url, "/")+1:]
	if _, err := b.wh.Receber(ctx, conta.ID, token, []byte(`{"pix":[]}`)); errors.Is(err, domain.ErrNaoAutenticado) {
		return fmt.Errorf("a URL informada respondeu 401: %w", err)
	}
	return b.recusar
}

func TestWebhookValeDuranteORegistroNoBanco(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	wh := &app.Webhooks{Store: c.store, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}}
	banco := &bancoQueChamaAURL{PSP: sandbox.New(c.clock), wh: wh}
	wh.PSPs = psp.NewRegistry(banco)

	endereco, err := wh.Configurar(ctx, "loja", "https://vintem.example.com")
	if err != nil {
		t.Fatalf("o banco não conseguiu chamar a URL durante o registro: %v", err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]

	// Se o banco recusar a URL nova, a antiga continua valendo lá, e aqui também.
	banco.recusar = errors.New("banco fora do ar")
	if _, err := wh.Configurar(ctx, "loja", "https://outra.example.com"); err == nil {
		t.Fatal("o banco recusou, e o erro sumiu")
	}
	if _, err := wh.Receber(ctx, "loja", token, []byte(`{"pix":[]}`)); errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("o token anterior deixou de valer depois da recusa: %v", err)
	}
}

// bancoQueFalhaAoRemover recusa o DELETE do webhook, como um banco fora do ar.
type bancoQueFalhaAoRemover struct {
	*sandbox.PSP
	falha error
}

func (b *bancoQueFalhaAoRemover) RemoverWebhook(ctx context.Context, conta domain.Conta) error {
	if b.falha != nil {
		return b.falha
	}
	return b.PSP.RemoverWebhook(ctx, conta)
}

// TestWebhookRemover (#178): cancela no banco e invalida o token; com o banco
// falhando, o token continua valendo.
func TestWebhookRemover(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	banco := &bancoQueFalhaAoRemover{PSP: sandbox.New(c.clock)}
	wh := app.Webhooks{Store: c.store, PSPs: psp.NewRegistry(banco), Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}}
	endereco, err := wh.Configurar(ctx, "loja", "https://vintem.example.com")
	if err != nil {
		t.Fatal(err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]
	vale := func() bool {
		_, err := wh.Receber(ctx, "loja", token, []byte(`{"pix":[]}`))
		return !errors.Is(err, domain.ErrNaoAutenticado)
	}

	banco.falha = errors.New("banco fora do ar")
	if _, err := wh.Remover(ctx, "loja"); err == nil || !errors.Is(err, app.ErrPSP) {
		t.Fatalf("banco falhando: err = %v", err)
	}
	if !vale() {
		t.Fatal("o banco falhou, e o token deixou de valer")
	}

	banco.falha = nil
	if registrado, err := wh.Remover(ctx, "loja"); err != nil || !registrado {
		t.Fatalf("Remover = %v, %v", registrado, err)
	}
	if vale() {
		t.Fatal("o token continuou valendo depois de remover")
	}
	// Sem nada no banco, remover de novo não é erro.
	if registrado, err := wh.Remover(ctx, "loja"); err != nil || registrado {
		t.Fatalf("remover de novo = %v, %v", registrado, err)
	}
	if _, err := wh.Remover(ctx, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("conta inexistente: err = %v", err)
	}
}
