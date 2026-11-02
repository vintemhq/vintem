// Package sandbox é o PSP de mentira do Vintém: implementa todas as
// capacidades sem banco nenhum, para testar o fluxo inteiro sem conta PJ (RN-52).
// É também o adapter de referência da suíte de conformidade.
//
// O comportamento é determinístico e controlado pelo valor da cobrança
// (os "valores mágicos", como os cartões de teste dos gateways):
//
//	R$ 0,66  o PSP recusa a criação (erro do banco)
//	R$ 0,99  nunca é paga: a cobrança expira
//	R$ 1,01  é paga com valor diferente (R$ 1,00), para testar divergência
//	R$ 2,02  é paga duas vezes (webhook duplicado)
//	outros   é paga integralmente depois do atraso configurado
//
// Com ComStore, o sandbox lembra das cobranças depois de reiniciar: o que não
// está na memória é reconstruído das cobranças que o Vintém gravou, que são o
// espelho do que o sandbox sabia (valor, location e criação bastam para os
// valores mágicos). As devoluções também: as que o Vintém gravou contam no saldo
// do Pix e respondem pelo id.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/pkg/brcode"
)

// Valores mágicos, em centavos.
const (
	ValorRecusado   domain.Money = 66
	ValorNuncaPaga  domain.Money = 99
	ValorDivergente domain.Money = 101
	ValorDuplicado  domain.Money = 202
)

// ErrRecusado é o erro que o sandbox devolve para ValorRecusado.
var ErrRecusado = errors.New("sandbox: o PSP recusou a cobrança (valor mágico 0.66)")

// AtrasoPadrao é quanto tempo a cobrança leva para ser paga.
const AtrasoPadrao = 5 * time.Second

// PSP é o adapter sandbox. O estado vive em memória.
type PSP struct {
	clock  ports.Clock
	atraso time.Duration
	store  ports.Store // opcional: de onde reconstruir as cobranças depois de reiniciar

	mu           sync.Mutex
	cobs         map[string]domain.Cobranca // chave: contaID/txid
	cobvs        map[string]domain.CobrancaVencimento
	devolucoes   map[string][]domain.Devolucao
	webhooks     map[string]string
	recs         map[string]domain.Recorrencia
	webhooksRec  map[string]string
	solicitadas  map[string]time.Time   // idRec → quando a confirmação foi pedida
	cobrs        map[string]cobrSandbox // chave: contaID/txid
	webhooksCobr map[string]string
	entregar     Entregador
	locSeq       int
}

var _ ports.PSP = (*PSP)(nil)

// Opcao configura o sandbox.
type Opcao func(*PSP)

// ComStore faz o sandbox buscar no store do Vintém as cobranças que não estão
// na memória, para que um restart não as esqueça.
func ComStore(s ports.Store) Opcao { return func(p *PSP) { p.store = s } }

// ComAtraso define em quanto tempo a cobrança é paga.
func ComAtraso(d time.Duration) Opcao { return func(p *PSP) { p.atraso = d } }

// New cria o sandbox com o relógio dado.
func New(clock ports.Clock, opcoes ...Opcao) *PSP {
	p := &PSP{clock: clock, atraso: AtrasoPadrao, cobs: map[string]domain.Cobranca{}, cobvs: map[string]domain.CobrancaVencimento{}, devolucoes: map[string][]domain.Devolucao{}, webhooks: map[string]string{}, recs: map[string]domain.Recorrencia{}, webhooksRec: map[string]string{}, solicitadas: map[string]time.Time{}, cobrs: map[string]cobrSandbox{}, webhooksCobr: map[string]string{}, entregar: entregarHTTP}
	for _, o := range opcoes {
		o(p)
	}
	return p
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "sandbox" }

// Capabilities declara tudo o que o sandbox simula hoje.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true, ports.CapWebhook: true, ports.CapRec: true, ports.CapCobr: true}
}

func chave(contaID string, txid domain.Txid) string { return contaID + "/" + string(txid) }

// CriarCob registra a cobrança e devolve location e BR Code dinâmico.
func (p *PSP) CriarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	if c.Valor == ValorRecusado {
		return domain.Cobranca{}, ErrRecusado
	}
	if err := p.lembrarCob(ctx, conta.ID, c.Txid); err != nil {
		return domain.Cobranca{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	k := chave(conta.ID, c.Txid)
	if _, ok := p.cobs[k]; ok {
		return domain.Cobranca{}, fmt.Errorf("sandbox: txid %s: %w", c.Txid, ports.ErrJaExiste)
	}
	p.locSeq++
	c.ContaID = conta.ID
	c.Status = domain.CobrancaAtiva
	c.Location = fmt.Sprintf("sandbox.vintem.dev/qr/v2/%s/%d", conta.ID, p.locSeq)
	if c.CriadaEm.IsZero() {
		c.CriadaEm = p.clock.Now()
	}
	code, err := brcode.Encode(brcode.Pix{URL: c.Location, Nome: nomeBRCode(conta.Nome), Cidade: "SANDBOX", Valor: int64(c.Valor)})
	if err != nil {
		return domain.Cobranca{}, fmt.Errorf("sandbox: gerando BR Code: %w", err)
	}
	c.BRCode = code
	p.cobs[k] = c
	return c, nil
}

// ConsultarCob devolve a cobrança, já CONCLUIDA se o atraso passou.
func (p *PSP) ConsultarCob(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error) {
	if err := p.lembrarCob(ctx, conta.ID, txid); err != nil {
		return domain.Cobranca{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.cobs[chave(conta.ID, txid)]
	if !ok {
		return domain.Cobranca{}, fmt.Errorf("sandbox: txid %s: %w", txid, ports.ErrNaoEncontrado)
	}
	if p.paga(c) {
		c.Status = domain.CobrancaConcluida
	}
	return c, nil
}

// RevisarCob aplica a revisão. Cobrança já paga ou removida não muda mais.
func (p *PSP) RevisarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	if err := p.lembrarCob(ctx, conta.ID, c.Txid); err != nil {
		return domain.Cobranca{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	k := chave(conta.ID, c.Txid)
	atual, ok := p.cobs[k]
	if !ok {
		return domain.Cobranca{}, fmt.Errorf("sandbox: txid %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if p.paga(atual) {
		atual.Status = domain.CobrancaConcluida
	}
	if c.Status != atual.Status {
		if err := atual.Transitar(c.Status); err != nil {
			return domain.Cobranca{}, fmt.Errorf("sandbox: %w", err)
		}
	} else if atual.Status != domain.CobrancaAtiva {
		return domain.Cobranca{}, fmt.Errorf("sandbox: cobrança %s está %s: %w", c.Txid, atual.Status, domain.ErrTransicaoInvalida)
	}
	atual.Valor, atual.Devedor, atual.Expiracao = c.Valor, c.Devedor, c.Expiracao
	atual.PermiteAlteracao, atual.SolicitacaoPagador = c.PermiteAlteracao, c.SolicitacaoPagador
	if atual.Status == domain.CobrancaAtiva {
		code, err := brcode.Encode(brcode.Pix{URL: atual.Location, Nome: nomeBRCode(conta.Nome), Cidade: "SANDBOX", Valor: int64(atual.Valor)})
		if err != nil {
			return domain.Cobranca{}, fmt.Errorf("sandbox: gerando BR Code: %w", err)
		}
		atual.BRCode = code
	}
	p.cobs[k] = atual
	return atual, nil
}

// CriarCobv registra a cobrança com vencimento e devolve location e BR Code.
func (p *PSP) CriarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	if c.Valor == ValorRecusado {
		return domain.CobrancaVencimento{}, ErrRecusado
	}
	if err := p.lembrarCobv(ctx, conta.ID, c.Txid); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	k := chave(conta.ID, c.Txid)
	if _, ok := p.cobvs[k]; ok {
		return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: txid %s: %w", c.Txid, ports.ErrJaExiste)
	}
	p.locSeq++
	c.ContaID, c.Status = conta.ID, domain.CobrancaAtiva
	c.Location = fmt.Sprintf("sandbox.vintem.dev/qr/v2/cobv/%s/%d", conta.ID, p.locSeq)
	if c.CriadaEm.IsZero() {
		c.CriadaEm = p.clock.Now()
	}
	code, err := brcode.Encode(brcode.Pix{URL: c.Location, Nome: nomeBRCode(conta.Nome), Cidade: "SANDBOX"})
	if err != nil {
		return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: gerando BR Code: %w", err)
	}
	c.BRCode = code
	p.cobvs[k] = c
	return c, nil
}

// ConsultarCobv devolve a cobv, CONCLUIDA se o atraso passou dentro da validade.
func (p *PSP) ConsultarCobv(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaVencimento, error) {
	if err := p.lembrarCobv(ctx, conta.ID, txid); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.cobvs[chave(conta.ID, txid)]
	if !ok {
		return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: txid %s: %w", txid, ports.ErrNaoEncontrado)
	}
	if p.pagaV(c) {
		c.Status = domain.CobrancaConcluida
	}
	return c, nil
}

// RevisarCobv aplica a revisão; cobv paga ou removida não muda mais.
func (p *PSP) RevisarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	if err := p.lembrarCobv(ctx, conta.ID, c.Txid); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	k := chave(conta.ID, c.Txid)
	atual, ok := p.cobvs[k]
	if !ok {
		return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: txid %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if p.pagaV(atual) {
		atual.Status = domain.CobrancaConcluida
	}
	if c.Status != atual.Status {
		if err := atual.Transitar(c.Status); err != nil {
			return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: %w", err)
		}
	} else if atual.Status != domain.CobrancaAtiva {
		return domain.CobrancaVencimento{}, fmt.Errorf("sandbox: cobrança %s está %s: %w", c.Txid, atual.Status, domain.ErrTransicaoInvalida)
	}
	status, location, code, criada := atual.Status, atual.Location, atual.BRCode, atual.CriadaEm
	atual = c
	atual.ContaID, atual.Status, atual.Location, atual.BRCode, atual.CriadaEm = conta.ID, status, location, code, criada
	p.cobvs[k] = atual
	return atual, nil
}

// pagaV diz se a cobv já foi paga: depois do atraso e até o último dia para pagar.
func (p *PSP) pagaV(c domain.CobrancaVencimento) bool {
	if c.Valor == ValorNuncaPaga || c.Status != domain.CobrancaAtiva {
		return false
	}
	pagoEm := c.CriadaEm.Add(p.atraso)
	return !p.clock.Now().Before(pagoEm) && !domain.DataDe(pagoEm).Depois(c.UltimoDiaParaPagar())
}

// paga diz se a cobrança já foi "paga" no instante atual do relógio.
func (p *PSP) paga(c domain.Cobranca) bool {
	if c.Valor == ValorNuncaPaga || c.Status != domain.CobrancaAtiva {
		return false
	}
	pagoEm := c.CriadaEm.Add(p.atraso)
	return !p.clock.Now().Before(pagoEm) && pagoEm.Before(c.CriadaEm.Add(c.Expiracao))
}

// ListarPix devolve os pagamentos simulados cujo horário cai na janela.
func (p *PSP) ListarPix(ctx context.Context, conta domain.Conta, j ports.Janela) ([]domain.PixRecebido, error) {
	if err := p.lembrarJanela(ctx, conta.ID, j); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []domain.PixRecebido
	for _, c := range p.cobs {
		if c.ContaID != conta.ID || !p.paga(c) {
			continue
		}
		horario := c.CriadaEm.Add(p.atraso)
		if horario.Before(j.Inicio) || !horario.Before(j.Fim) {
			continue
		}
		valor := c.Valor
		if valor == ValorDivergente {
			valor = 100
		}
		vezes := 1
		if c.Valor == ValorDuplicado {
			vezes = 2
		}
		pix := domain.PixRecebido{EndToEndID: endToEndID(horario, c.Txid), ContaID: conta.ID, Txid: c.Txid, Valor: valor, Horario: horario, Pagador: &domain.Devedor{Documento: "12345678909", Nome: "Pagador Sandbox"}}
		for range vezes {
			out = append(out, pix)
		}
	}
	for _, c := range p.cobvs {
		if c.ContaID != conta.ID || !p.pagaV(c) {
			continue
		}
		horario := c.CriadaEm.Add(p.atraso)
		if horario.Before(j.Inicio) || !horario.Before(j.Fim) {
			continue
		}
		// O sandbox paga o valor original: multa, juros e desconto são calculados pelo banco de verdade.
		out = append(out, domain.PixRecebido{EndToEndID: endToEndID(horario, c.Txid), ContaID: conta.ID, Txid: c.Txid, Valor: c.Valor, Horario: horario, Pagador: &c.Devedor.Devedor})
	}
	return out, nil
}

// Devolver aceita devoluções até o valor do Pix e responde DEVOLVIDO na hora,
// com rtrId e horário de liquidação.
func (p *PSP) Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error) {
	pixes, err := p.ListarPix(ctx, conta, ports.Janela{Fim: p.clock.Now().Add(time.Nanosecond)})
	if err != nil {
		return domain.Devolucao{}, err
	}
	var recebido domain.Money
	for _, px := range pixes {
		if px.EndToEndID == d.EndToEndID {
			recebido = px.Valor
			break
		}
	}
	if recebido == 0 {
		return domain.Devolucao{}, fmt.Errorf("sandbox: pix %s: %w", d.EndToEndID, ports.ErrNaoEncontrado)
	}
	anteriores, err := p.devolucoesDoPix(ctx, d.EndToEndID)
	if err != nil {
		return domain.Devolucao{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var devolvido domain.Money
	for _, ant := range anteriores {
		if ant.ID == d.ID {
			return ant, nil // idempotente pelo id
		}
		devolvido += ant.Valor
	}
	if devolvido+d.Valor > recebido {
		return domain.Devolucao{}, fmt.Errorf("sandbox: devolução de %s excede o saldo de %s", d.Valor, recebido-devolvido)
	}
	agora := p.clock.Now()
	d.ContaID, d.Status, d.CriadaEm, d.LiquidadaEm = conta.ID, domain.DevolucaoDevolvido, agora, agora
	d.RtrID = "D" + ispbSandbox + agora.UTC().Format("200601021504") + sufixo11()
	p.devolucoes[d.EndToEndID] = append(p.devolucoes[d.EndToEndID], d)
	return d, nil
}

// ConsultarDevolucao acha a devolução na memória ou no que o Vintém gravou.
func (p *PSP) ConsultarDevolucao(ctx context.Context, conta domain.Conta, e2e, id string) (domain.Devolucao, error) {
	ds, err := p.devolucoesDoPix(ctx, e2e)
	if err != nil {
		return domain.Devolucao{}, err
	}
	for _, d := range ds {
		if d.ID == id && d.ContaID == conta.ID {
			return d, nil
		}
	}
	return domain.Devolucao{}, fmt.Errorf("sandbox: devolução %s do Pix %s: %w", id, e2e, ports.ErrNaoEncontrado)
}

// devolucoesDoPix junta as devoluções da memória com as que o Vintém gravou,
// que sobrevivem a um restart do sandbox.
func (p *PSP) devolucoesDoPix(ctx context.Context, e2e string) ([]domain.Devolucao, error) {
	p.mu.Lock()
	out := slices.Clone(p.devolucoes[e2e])
	p.mu.Unlock()
	if p.store == nil {
		return out, nil
	}
	gravadas, err := p.store.Devolucoes().DoPix(ctx, e2e)
	if err != nil {
		return nil, fmt.Errorf("sandbox: lembrando as devoluções do Pix %s: %w", e2e, err)
	}
	for _, g := range gravadas {
		if !slices.ContainsFunc(out, func(d domain.Devolucao) bool { return d.ID == g.ID }) {
			out = append(out, g)
		}
	}
	return out, nil
}

// ConfigurarWebhook guarda a URL. A entrega dos avisos chega com a F4.
func (p *PSP) ConfigurarWebhook(_ context.Context, conta domain.Conta, url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.webhooks[conta.ID] = url
	return nil
}

// RemoverWebhook esquece a URL; sem URL guardada, ErrNaoEncontrado, como o banco.
func (p *PSP) RemoverWebhook(_ context.Context, conta domain.Conta) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.webhooks[conta.ID]; !ok {
		return fmt.Errorf("webhook da conta %s: %w", conta.ID, ports.ErrNaoEncontrado)
	}
	delete(p.webhooks, conta.ID)
	return nil
}

// InterpretarWebhook lê o callback no formato da spec do BCB.
func (p *PSP) InterpretarWebhook(conta domain.Conta, corpo []byte) ([]domain.PixRecebido, error) {
	return psp.WebhookPixBCB(conta, corpo)
}

// endToEndID segue o formato do SPI: E + ISPB(8) + AAAAMMDDHHmm + 11 caracteres.
// É derivado do txid para que consultas repetidas devolvam o mesmo id.
func endToEndID(t time.Time, txid domain.Txid) string {
	sufixo := strings.ToUpper(string(txid))
	if len(sufixo) > 11 {
		sufixo = sufixo[len(sufixo)-11:]
	}
	for len(sufixo) < 11 {
		sufixo = "0" + sufixo
	}
	return "E00000000" + t.UTC().Format("200601021504") + sufixo
}

func nomeBRCode(nome string) string {
	if len(nome) > 25 {
		return nome[:25]
	}
	if nome == "" {
		return "VINTEM SANDBOX"
	}
	return nome
}

// lembrar põe na memória as cobranças do store que o sandbox ainda não conhece.
// No store, a cobrança paga está CONCLUIDA; no sandbox ela fica ATIVA e o
// pagamento é calculado pelo horário, como antes do restart.
func (p *PSP) lembrarCob(ctx context.Context, contaID string, txid domain.Txid) error {
	p.mu.Lock()
	_, ok := p.cobs[chave(contaID, txid)]
	p.mu.Unlock()
	if ok || p.store == nil {
		return nil
	}
	c, err := p.store.Cobrancas().Buscar(ctx, contaID, txid)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sandbox: lembrando a cobrança %s: %w", txid, err)
	}
	p.guardarCob(c)
	return nil
}

func (p *PSP) lembrarCobv(ctx context.Context, contaID string, txid domain.Txid) error {
	p.mu.Lock()
	_, ok := p.cobvs[chave(contaID, txid)]
	p.mu.Unlock()
	if ok || p.store == nil {
		return nil
	}
	c, err := p.store.CobrancasV().BuscarPorTxid(ctx, txid)
	if errors.Is(err, ports.ErrNaoEncontrado) || (err == nil && c.ContaID != contaID) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("sandbox: lembrando a cobv %s: %w", txid, err)
	}
	p.guardarCobv(c)
	return nil
}

func (p *PSP) guardarCob(c domain.Cobranca) {
	if c.Status == domain.CobrancaConcluida {
		c.Status = domain.CobrancaAtiva
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.cobs[chave(c.ContaID, c.Txid)]; !ok {
		p.cobs[chave(c.ContaID, c.Txid)] = c
	}
}

func (p *PSP) guardarCobv(c domain.CobrancaVencimento) {
	if c.Status == domain.CobrancaConcluida {
		c.Status = domain.CobrancaAtiva
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.cobvs[chave(c.ContaID, c.Txid)]; !ok {
		p.cobvs[chave(c.ContaID, c.Txid)] = c
	}
}

// lembrarJanela traz do store as cobranças da conta cujo pagamento simulado cai
// na janela, para o ListarPix depois de reiniciar.
func (p *PSP) lembrarJanela(ctx context.Context, contaID string, j ports.Janela) error {
	if p.store == nil {
		return nil
	}
	// O Pix cai em CriadaEm + atraso: a criação fica em [Inicio-atraso, Fim-atraso).
	criacao := ports.Janela{Inicio: j.Inicio.Add(-p.atraso), Fim: j.Fim.Add(-p.atraso)}
	if j.Inicio.IsZero() {
		criacao.Inicio = time.Time{}
	}
	f := ports.FiltroCobrancas{ContaID: contaID, Janela: criacao, Limite: 500}
	for {
		cobs, cursor, err := p.store.Cobrancas().Listar(ctx, f)
		if err != nil {
			return fmt.Errorf("sandbox: lembrando cobranças: %w", err)
		}
		for _, c := range cobs {
			p.guardarCob(c)
		}
		if cursor == "" {
			break
		}
		f.Cursor = cursor
	}
	f.Cursor = ""
	for {
		cobvs, cursor, err := p.store.CobrancasV().Listar(ctx, f)
		if err != nil {
			return fmt.Errorf("sandbox: lembrando cobvs: %w", err)
		}
		for _, c := range cobvs {
			p.guardarCobv(c)
		}
		if cursor == "" {
			return nil
		}
		f.Cursor = cursor
	}
}
