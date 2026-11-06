// Package conformance é a suíte que todo adapter de PSP precisa passar (RN-51).
// Os cenários respeitam as capacidades declaradas: o que o adapter não declara
// é pulado, e o que ele declara precisa funcionar como a spec manda.
package conformance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/pkg/brcode"
)

// Alvo é o adapter sob teste e o que a suíte precisa para exercitá-lo.
type Alvo struct {
	PSP   ports.PSP
	Conta domain.Conta
	// Agora é o instante "atual" do alvo, para montar janelas de consulta.
	Agora func() time.Time
	// Pagar faz a cobrança ser paga. Só o sandbox e fixtures gravadas conseguem;
	// nil pula os cenários de pagamento.
	Pagar func(t *testing.T, txid domain.Txid)
	// Rodada entra em todo txid criado pela suíte. Contra um banco de verdade
	// cada gravação precisa de uma rodada nova, porque o banco não esquece as
	// cobranças da gravação anterior. Vazia vale "00000000".
	Rodada string
	// Real marca um banco de verdade (gravando ou reproduzindo fitas): o que
	// só um servidor falso aceita é pulado.
	Real bool
	// URLWebhook é uma URL pública que responde 2xx. O banco de verdade testa
	// a URL antes de aceitar o webhook; vazia, usa um endereço de exemplo.
	URLWebhook string
}

// Run executa a suíte. novo deve devolver um alvo limpo a cada chamada.
func Run(t *testing.T, novo func(t *testing.T) Alvo) {
	t.Run("declara nome e capacidades", func(t *testing.T) {
		a := novo(t)
		if a.PSP.Nome() == "" {
			t.Fatal("Nome() vazio")
		}
		if len(a.PSP.Capabilities()) == 0 {
			t.Fatal("nenhuma capacidade declarada")
		}
	})

	t.Run("cob: criar devolve ATIVA, location e BR Code válido", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		c, err := a.PSP.CriarCob(context.Background(), a.Conta, novaCob(a, txid(a, "cob001")))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != domain.CobrancaAtiva || c.Location == "" {
			t.Fatalf("status=%s location=%q", c.Status, c.Location)
		}
		p, err := brcode.Decode(c.BRCode)
		if err != nil {
			t.Fatalf("BR Code inválido: %v", err)
		}
		if !p.Dinamico() || p.URL != c.Location {
			t.Fatalf("BR Code deveria ser dinâmico apontando para a location: %+v", p)
		}
	})

	t.Run("cob: txid repetido é recusado", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		ctx := context.Background()
		if _, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, txid(a, "cob002"))); err != nil {
			t.Fatal(err)
		}
		if _, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, txid(a, "cob002"))); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("err = %v, quer ErrJaExiste", err)
		}
	})

	t.Run("cob: consultar inexistente é ErrNaoEncontrado", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if _, err := a.PSP.ConsultarCob(context.Background(), a.Conta, "conformidadeinexistente0001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("cob: revisar altera o valor e remover encerra", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		ctx := context.Background()
		c, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, txid(a, "cob005")))
		if err != nil {
			t.Fatal(err)
		}
		c.Valor = 3000
		rev, err := a.PSP.RevisarCob(ctx, a.Conta, c)
		if err != nil || rev.Valor != 3000 || rev.Status != domain.CobrancaAtiva {
			t.Fatalf("revisão de valor: %+v, %v", rev, err)
		}
		rev.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if rev, err = a.PSP.RevisarCob(ctx, a.Conta, rev); err != nil || rev.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("remoção: %+v, %v", rev, err)
		}
		rev.Valor = 1
		if _, err := a.PSP.RevisarCob(ctx, a.Conta, rev); err == nil {
			t.Fatal("aceitou revisar cobrança removida")
		}
	})

	t.Run("cobv: criar, consultar, revisar e remover", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCobv)
		ctx := context.Background()
		v := novaCobv(a, txid(a, "cobv01"))
		c, err := a.PSP.CriarCobv(ctx, a.Conta, v)
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != domain.CobrancaAtiva || c.Location == "" || c.Vencimento != v.Vencimento {
			t.Fatalf("criada: %+v", c)
		}
		if p, err := brcode.Decode(c.BRCode); err != nil || p.URL != c.Location {
			t.Fatalf("BR Code: %+v, %v", p, err)
		}
		if _, err := a.PSP.CriarCobv(ctx, a.Conta, v); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("txid repetido: err = %v", err)
		}
		if _, err := a.PSP.ConsultarCobv(ctx, a.Conta, "conformidadevinexistente0001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
		c.Valor = 20000
		rev, err := a.PSP.RevisarCobv(ctx, a.Conta, c)
		if err != nil || rev.Valor != 20000 {
			t.Fatalf("revisão: %+v, %v", rev, err)
		}
		rev.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if rev, err = a.PSP.RevisarCobv(ctx, a.Conta, rev); err != nil || rev.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("remoção: %+v, %v", rev, err)
		}
	})

	t.Run("pagamento: cobv paga dentro da validade vira CONCLUIDA", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCobv)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		c, err := a.PSP.CriarCobv(ctx, a.Conta, novaCobvPaga(a, txid(a, "cobv02")))
		if err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		if got, err := a.PSP.ConsultarCobv(ctx, a.Conta, c.Txid); err != nil || got.Status != domain.CobrancaConcluida {
			t.Fatalf("depois de pagar: %s, %v", got.Status, err)
		}
	})

	t.Run("pagamento: cob removida não é paga", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		c, _ := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, txid(a, "cob006")))
		c.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if _, err := a.PSP.RevisarCob(ctx, a.Conta, c); err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		if got, _ := a.PSP.ConsultarCob(ctx, a.Conta, c.Txid); got.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("cobrança removida virou %s", got.Status)
		}
	})

	t.Run("pagamento: cob paga vira CONCLUIDA e aparece em ListarPix", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		inicio := a.Agora()
		c, err := a.PSP.CriarCob(ctx, a.Conta, novaCobPaga(a, txid(a, "cob003")))
		if err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		got, err := a.PSP.ConsultarCob(ctx, a.Conta, c.Txid)
		if err != nil || got.Status != domain.CobrancaConcluida {
			t.Fatalf("depois de pagar: status=%s err=%v", got.Status, err)
		}
		pix, err := a.PSP.ListarPix(ctx, a.Conta, ports.Janela{Inicio: inicio, Fim: a.Agora().Add(time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		// Contra um banco de verdade a janela pode ter Pix de outros cenários:
		// o que importa é o desta cobrança.
		daCob := doTxid(pix, c.Txid)
		if len(daCob) != 1 || daCob[0].Valor != c.Valor || len(daCob[0].EndToEndID) != 32 {
			t.Fatalf("ListarPix, Pix da cobrança %s: %+v", c.Txid, pix)
		}
	})

	t.Run("devolução: não passa do valor recebido e é idempotente pelo id", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapDevolucao)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		inicio := a.Agora()
		c, _ := a.PSP.CriarCob(ctx, a.Conta, novaCobPaga(a, txid(a, "cob004")))
		a.Pagar(t, c.Txid)
		pix, _ := a.PSP.ListarPix(ctx, a.Conta, ports.Janela{Inicio: inicio, Fim: a.Agora().Add(time.Second)})
		daCob := doTxid(pix, c.Txid)
		if len(daCob) == 0 {
			t.Fatal("pagamento não apareceu")
		}
		e2e := daCob[0].EndToEndID

		d, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 200})
		if err != nil || d.Status == "" {
			t.Fatalf("primeira devolução: %+v, %v", d, err)
		}
		if _, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 200}); err != nil {
			t.Fatalf("repetir o mesmo id deveria ser idempotente: %v", err)
		}
		if _, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d2", EndToEndID: e2e, Valor: c.Valor}); err == nil {
			t.Fatal("aceitou devolver mais do que o recebido")
		}
		if lida, err := a.PSP.ConsultarDevolucao(ctx, a.Conta, e2e, "d1"); err != nil || lida.ID != "d1" || lida.Valor != 200 || lida.Status == "" {
			t.Fatalf("consultar devolução: %+v, %v", lida, err)
		}
		if _, err := a.PSP.ConsultarDevolucao(ctx, a.Conta, e2e, "inexistente"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("devolução inexistente: err = %v", err)
		}
	})

	t.Run("rec: criar devolve CRIADA com idRec da spec, e a solicitação tem idSolicRec", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapRec)
		rec, ok := a.PSP.(ports.PSPRecorrencia)
		if !ok {
			t.Fatal("declara rec mas não implementa ports.PSPRecorrencia")
		}
		ctx := context.Background()
		r := domain.Recorrencia{
			ContaID: a.Conta.ID, Contrato: "conformidade", Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa de Teste"},
			DataInicial: domain.DataDe(a.Agora()).SomarDias(10), Periodicidade: domain.Mensal, ValorFixo: 1500, Politica: domain.Retentativa3Em7Dias,
		}
		criada, err := rec.CriarRecorrencia(ctx, a.Conta, r)
		if err != nil {
			t.Fatal(err)
		}
		if err := domain.ValidarIDRecorrencia(criada.ID); err != nil || criada.Status != domain.RecorrenciaCriada || criada.ID[1] != 'R' {
			t.Fatalf("criada: id %q status %s (%v); com PERMITE_3R_7D o 2º caractere é R", criada.ID, criada.Status, err)
		}
		if lida, err := rec.ConsultarRecorrencia(ctx, a.Conta, criada.ID); err != nil || lida.ID != criada.ID {
			t.Fatalf("consultar: %+v, %v", lida, err)
		}
		sol, err := rec.SolicitarConfirmacao(ctx, a.Conta, domain.SolicitacaoRecorrencia{
			RecorrenciaID: criada.ID, ContaID: a.Conta.ID, Destinatario: domain.ContaPagador{ISPB: "18236120", Conta: "123456"}, ExpiraEm: a.Agora().Add(24 * time.Hour),
		})
		if err != nil || len(sol.ID) != 29 || sol.ID[:2] != "SC" {
			t.Fatalf("solicitação: %+v, %v", sol, err)
		}
		if _, err := rec.ConsultarRecorrencia(ctx, a.Conta, "RR0000000020261029inexistent"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("cobr: criar devolve CRIADA, consultar acha, cancelar encerra", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapRec)
		exigir(t, a, ports.CapCobr)
		rec, ok := a.PSP.(ports.PSPRecorrencia)
		if !ok {
			t.Fatal("declara rec mas não implementa ports.PSPRecorrencia")
		}
		cobr, ok := a.PSP.(ports.PSPCobrancaRecorrente)
		if !ok {
			t.Fatal("declara cobr mas não implementa ports.PSPCobrancaRecorrente")
		}
		ctx := context.Background()
		hoje := domain.DataDe(a.Agora())
		r, err := rec.CriarRecorrencia(ctx, a.Conta, domain.Recorrencia{
			ContaID: a.Conta.ID, Contrato: "conformidade-cobr", Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa de Teste"},
			DataInicial: hoje.SomarDias(5), Periodicidade: domain.Mensal, ValorFixo: 1500, Politica: domain.Retentativa3Em7Dias,
		})
		if err != nil {
			t.Fatal(err)
		}
		c, err := cobr.CriarCobr(ctx, a.Conta, domain.CobrancaRecorrente{
			Txid: domain.Txid(txid(a, "cobr01")), RecorrenciaID: r.ID, ContaID: a.Conta.ID, Valor: 1500,
			Vencimento: hoje.SomarDias(5), AjusteDiaUtil: true, CriadaEm: a.Agora(),
		})
		if err != nil || c.Status != domain.CobrCriada || string(c.Txid) != txid(a, "cobr01") {
			t.Fatalf("criar: %+v, %v", c, err)
		}
		if lida, err := cobr.ConsultarCobr(ctx, a.Conta, c.Txid); err != nil || lida.Txid != c.Txid || lida.RecorrenciaID != r.ID {
			t.Fatalf("consultar: %+v, %v", lida, err)
		}
		if cancelada, err := cobr.CancelarCobr(ctx, a.Conta, c.Txid); err != nil || cancelada.Status != domain.CobrCancelada {
			t.Fatalf("cancelar: %+v, %v", cancelada, err)
		}
		if _, err := cobr.SolicitarRetentativa(ctx, a.Conta, c.Txid, hoje.SomarDias(7)); err == nil {
			t.Fatal("aceitou retentativa de cobrança cancelada")
		}
		if _, err := cobr.ConsultarCobr(ctx, a.Conta, "conformidadecobrinexistente1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
		if err := cobr.ConfigurarWebhookCobr(ctx, a.Conta, "https://vintem.example.com/webhooks/cobr/"+a.Conta.ID+"/token"); err != nil {
			t.Fatalf("configurar webhookcobr: %v", err)
		}
	})

	t.Run("webhook: configurar URL", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapWebhook)
		url := a.URLWebhook
		if url == "" {
			if a.Real {
				t.Skip("o banco testa a URL do webhook antes de aceitar: o alvo precisa de URLWebhook pública")
			}
			url = "https://vintem.example.com/webhook/" + a.Conta.ID
		}
		if err := a.PSP.ConfigurarWebhook(context.Background(), a.Conta, url); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("webhook: remover a URL registrada", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapWebhook)
		url := a.URLWebhook
		if url == "" {
			if a.Real {
				t.Skip("o banco testa a URL do webhook antes de aceitar: o alvo precisa de URLWebhook pública")
			}
			url = "https://vintem.example.com/webhook/" + a.Conta.ID
		}
		ctx := context.Background()
		if err := a.PSP.ConfigurarWebhook(ctx, a.Conta, url); err != nil {
			t.Fatal(err)
		}
		if err := a.PSP.RemoverWebhook(ctx, a.Conta); err != nil {
			t.Fatalf("remover: %v", err)
		}
		// Sem webhook registrado, o banco responde 404: o adapter traduz para
		// ErrNaoEncontrado, e o Vintém trata como já removido.
		if err := a.PSP.RemoverWebhook(ctx, a.Conta); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("remover de novo: err = %v, quer ErrNaoEncontrado", err)
		}
	})

	t.Run("webhook: interpreta o aviso de Pix no formato da spec", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapWebhook)
		corpo := `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","txid":"` + txid(a, "cob007") + `","valor":"49.90",
		  "chave":"` + a.Conta.ChavePix + `","horario":"2026-10-29T12:00:05-03:00","infoPagador":"obrigado"}]}`
		pix, err := a.PSP.InterpretarWebhook(a.Conta, []byte(corpo))
		if err != nil {
			t.Fatal(err)
		}
		if len(pix) != 1 || pix[0].EndToEndID != "E12345678202610291200abcdefghijk" || string(pix[0].Txid) != txid(a, "cob007") ||
			pix[0].Valor != 4990 || pix[0].ContaID != a.Conta.ID || pix[0].Horario.IsZero() {
			t.Fatalf("pix = %+v", pix)
		}
		if _, err := a.PSP.InterpretarWebhook(a.Conta, []byte(`{"pix":`)); err == nil {
			t.Fatal("aceitou corpo quebrado")
		}
	})

	t.Run("webhook: interpreta os avisos de rec e cobr no formato da spec", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapRec)
		rec, ok := a.PSP.(ports.PSPRecorrencia)
		if !ok {
			t.Fatal("declara rec mas não implementa ports.PSPRecorrencia")
		}
		ctx := context.Background()
		if err := rec.ConfigurarWebhookRec(ctx, a.Conta, "https://vintem.example.com/webhooks/rec/"+a.Conta.ID+"/token"); err != nil {
			t.Fatalf("configurar webhookrec: %v", err)
		}
		ats, err := rec.InterpretarWebhookRec(a.Conta, []byte(`{"recs":[{"idRec":"RR1026652320240821lab77511abf","status":"APROVADA",
		  "atualizacao":[{"status":"CRIADA","data":"2024-08-20T10:12:07.567Z"},{"status":"APROVADA","data":"2024-08-22T12:43:53.337Z"}]}]}`))
		if err != nil || len(ats) != 1 || ats[0].IDRec != "RR1026652320240821lab77511abf" || ats[0].Status != domain.RecorrenciaAprovada {
			t.Fatalf("rec: %+v, %v", ats, err)
		}
		if a.PSP.Capabilities().Suporta(ports.CapCobr) != nil {
			return
		}
		cobr, ok := a.PSP.(ports.PSPCobrancaRecorrente)
		if !ok {
			t.Fatal("declara cobr mas não implementa ports.PSPCobrancaRecorrente")
		}
		atc, err := cobr.InterpretarWebhookCobr(a.Conta, []byte(`{"cobsr":[{"idRec":"RR1026652320240821lab77511abf","txid":"`+txid(a, "cobr02")+`",
		  "status":"ATIVA","atualizacao":[{"status":"ATIVA","data":"2024-08-20T12:34:21.300Z"}],
		  "tentativas":[{"dataLiquidacao":"2024-08-20","tipo":"AGND","status":"SOLICITADA","endToEndId":"E12345678202406201221abcdef12345"}],
		  "valor":{"original":"100.00"}}]}`))
		if err != nil || len(atc) != 1 || string(atc[0].Txid) != txid(a, "cobr02") || atc[0].Status != domain.CobrAtiva || len(atc[0].Tentativas) != 1 {
			t.Fatalf("cobr: %+v, %v", atc, err)
		}
	})

	t.Run("capacidades: o que é declarado está implementado, o resto responde ErrNaoSuportado", func(t *testing.T) {
		a := novo(t)
		caps := a.PSP.Capabilities()
		if _, ok := a.PSP.(ports.PSPRecorrencia); caps[ports.CapRec] && !ok {
			t.Error("declara rec mas não implementa ports.PSPRecorrencia")
		}
		if _, ok := a.PSP.(ports.PSPCobrancaRecorrente); caps[ports.CapCobr] && !ok {
			t.Error("declara cobr mas não implementa ports.PSPCobrancaRecorrente")
		}
		if caps[ports.CapCobr] && !caps[ports.CapRec] {
			t.Error("declara cobr sem rec: a cobrança recorrente precisa de uma recorrência")
		}
		ctx := context.Background()
		naoDeclaradas := map[ports.Capacidade]func() error{
			ports.CapCob: func() error {
				_, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, txid(a, "cob099")))
				return err
			},
			ports.CapCobv: func() error {
				_, err := a.PSP.CriarCobv(ctx, a.Conta, novaCobv(a, txid(a, "cobv99")))
				return err
			},
			ports.CapDevolucao: func() error {
				_, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d1", EndToEndID: "E12345678202610291200abcdefghijk", Valor: 100})
				return err
			},
			ports.CapWebhook: func() error {
				return a.PSP.ConfigurarWebhook(ctx, a.Conta, "https://vintem.example.com/webhook/"+a.Conta.ID)
			},
		}
		for c, chamar := range naoDeclaradas {
			if caps[c] {
				continue
			}
			if err := chamar(); !errors.Is(err, ports.ErrNaoSuportado) {
				t.Errorf("%s não foi declarada, mas a chamada devolveu %v em vez de ErrNaoSuportado", c, err)
			}
		}
	})
}

// doTxid filtra os Pix de uma cobrança.
func doTxid(pix []domain.PixRecebido, txid domain.Txid) []domain.PixRecebido {
	var out []domain.PixRecebido
	for _, p := range pix {
		if p.Txid == txid {
			out = append(out, p)
		}
	}
	return out
}

func exigir(t *testing.T, a Alvo, c ports.Capacidade) {
	t.Helper()
	if err := a.PSP.Capabilities().Suporta(c); err != nil {
		t.Skipf("adapter não declara %s", c)
	}
}

// txid monta um txid de 26 caracteres: "conformidade" + rodada + sufixo de 6.
func txid(a Alvo, sufixo string) string {
	rodada := a.Rodada
	if rodada == "" {
		rodada = "00000000"
	}
	return "conformidade" + rodada + sufixo
}

// Valores da suíte. A homologação do Efí paga sozinha, em segundos, qualquer
// cobrança de até R$ 10,00. Por isso a cobrança que precisa continuar ATIVA
// vale mais que isso, e só os cenários de pagamento usam valorPago. Todos
// ficam longe dos valores mágicos do sandbox (0,66, 0,99, 1,01, 2,02).
const valorPago domain.Money = 500

// novaCobPaga e novaCobvPaga são as cobranças que o alvo consegue fazer ser pagas.
func novaCobPaga(a Alvo, txid string) domain.Cobranca {
	c := novaCob(a, txid)
	c.Valor = valorPago
	return c
}

func novaCobvPaga(a Alvo, txid string) domain.CobrancaVencimento {
	c := novaCobv(a, txid)
	c.Valor = valorPago
	return c
}

func novaCobv(a Alvo, txid string) domain.CobrancaVencimento {
	return domain.CobrancaVencimento{
		Txid: domain.Txid(txid), ContaID: a.Conta.ID, Valor: 15000, Vencimento: domain.DataDe(a.Agora()).SomarDias(10),
		ValidadeAposVencimento: 30, CriadaEm: a.Agora(),
		Devedor: domain.DevedorCompleto{Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa de Teste"}},
		Multa:   &domain.ValorModalidade{Modalidade: 2, ValorPerc: 200},
	}
}

func novaCob(a Alvo, txid string) domain.Cobranca {
	return domain.Cobranca{
		Txid: domain.Txid(txid), ContaID: a.Conta.ID, Valor: 2500, Expiracao: domain.ExpiracaoPadrao,
		CriadaEm: a.Agora(), SolicitacaoPagador: "Teste de conformidade",
	}
}
