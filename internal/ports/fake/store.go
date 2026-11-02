package fake

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

type dados struct {
	contas    map[string]domain.Conta
	creds     map[string][]byte
	tokens    map[string][]byte
	pix       map[string]domain.PixRecebido
	cobrancas map[string]domain.Cobranca // chave: contaID + "/" + txid
	ordem     []string                   // chaves de cobrança em ordem de criação
	cobvs     map[domain.Txid]domain.CobrancaVencimento
	ordemV    []domain.Txid
	outbox    []domain.Evento
	distrib   map[string]bool
	destinos  map[string]domain.Destino
	entregas  map[[2]string]domain.Entrega
	apikeys   map[string]domain.APIKey
	idem      map[[2]string]domain.RegistroIdempotencia
	recs      map[string]domain.Recorrencia
	solics    map[string]domain.SolicitacaoRecorrencia
	cobrs     map[domain.Txid]domain.CobrancaRecorrente
	devs      map[[2]string]domain.Devolucao // chave: e2e, id
}

func (d *dados) clonar() *dados {
	return &dados{
		contas:    maps.Clone(d.contas),
		creds:     maps.Clone(d.creds),
		tokens:    maps.Clone(d.tokens),
		pix:       maps.Clone(d.pix),
		cobrancas: maps.Clone(d.cobrancas),
		ordem:     slices.Clone(d.ordem),
		cobvs:     maps.Clone(d.cobvs),
		ordemV:    slices.Clone(d.ordemV),
		outbox:    slices.Clone(d.outbox),
		distrib:   maps.Clone(d.distrib),
		destinos:  maps.Clone(d.destinos),
		entregas:  maps.Clone(d.entregas),
		apikeys:   maps.Clone(d.apikeys),
		idem:      maps.Clone(d.idem),
		recs:      maps.Clone(d.recs),
		solics:    maps.Clone(d.solics),
		cobrs:     maps.Clone(d.cobrs),
		devs:      maps.Clone(d.devs),
	}
}

// Store é um ports.Store em memória. WithTx trabalha numa cópia e só a
// publica se fn não falhar, o que reproduz o rollback de um banco real.
type Store struct {
	mu   *sync.Mutex
	d    *dados
	emTx bool
}

var _ ports.Store = (*Store)(nil)

// NewStore cria um Store vazio.
func NewStore() *Store {
	return &Store{mu: &sync.Mutex{}, d: &dados{contas: map[string]domain.Conta{}, creds: map[string][]byte{}, tokens: map[string][]byte{}, pix: map[string]domain.PixRecebido{}, cobrancas: map[string]domain.Cobranca{}, apikeys: map[string]domain.APIKey{}, cobvs: map[domain.Txid]domain.CobrancaVencimento{}, idem: map[[2]string]domain.RegistroIdempotencia{}, distrib: map[string]bool{}, destinos: map[string]domain.Destino{}, entregas: map[[2]string]domain.Entrega{}, recs: map[string]domain.Recorrencia{}, solics: map[string]domain.SolicitacaoRecorrencia{}, cobrs: map[domain.Txid]domain.CobrancaRecorrente{}, devs: map[[2]string]domain.Devolucao{}}}
}

// Eventos devolve o que foi publicado no outbox, para asserções.
func (s *Store) Eventos() []domain.Evento {
	s.lock()
	defer s.unlock()
	return slices.Clone(s.d.outbox)
}

func (s *Store) lock() {
	if !s.emTx {
		s.mu.Lock()
	}
}

func (s *Store) unlock() {
	if !s.emTx {
		s.mu.Unlock()
	}
}

// WithTx executa fn sobre uma cópia isolada dos dados.
func (s *Store) WithTx(_ context.Context, fn func(ports.Store) error) error {
	if s.emTx {
		return fn(s)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := &Store{mu: s.mu, d: s.d.clonar(), emTx: true}
	if err := fn(tx); err != nil {
		return err
	}
	s.d = tx.d
	return nil
}

// Contas devolve o repositório de contas.
func (s *Store) Contas() ports.ContaRepo { return contas{s} }

// Cobrancas devolve o repositório de cobranças.
func (s *Store) Cobrancas() ports.CobrancaRepo { return cobrancas{s} }

// APIKeys devolve o repositório de API keys.
func (s *Store) APIKeys() ports.APIKeyRepo { return apikeys{s} }

// Idempotencia devolve o repositório do Idempotency-Key.
func (s *Store) Idempotencia() ports.IdempotenciaRepo { return idem{s} }

// CobrancasV devolve o repositório de cobranças com vencimento.
func (s *Store) CobrancasV() ports.CobrancaVRepo { return cobvs{s} }

// Pix devolve o repositório de Pix recebidos.
func (s *Store) Pix() ports.PixRepo { return pixes{s} }

// Destinos devolve o repositório de destinos de webhook.
func (s *Store) Destinos() ports.DestinoRepo { return destinos{s} }

// Entregas devolve o repositório de entregas de webhook.
func (s *Store) Entregas() ports.EntregaRepo { return entregas{s} }

// Outbox devolve o repositório do outbox.
func (s *Store) Outbox() ports.OutboxRepo { return outbox{s} }

type contas struct{ s *Store }

func (r contas) Criar(_ context.Context, c domain.Conta) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[c.ID]; ok {
		return fmt.Errorf("conta %s: %w", c.ID, ports.ErrJaExiste)
	}
	for _, outra := range r.s.d.contas {
		if outra.ChavePix == c.ChavePix {
			return fmt.Errorf("chave %s já pertence à conta %s: %w", c.ChavePix, outra.ID, ports.ErrJaExiste)
		}
	}
	r.s.d.contas[c.ID] = c
	return nil
}

func (r contas) BuscarPorChave(_ context.Context, chave string) (domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	for _, c := range r.s.d.contas {
		if c.ChavePix == chave {
			return c, nil
		}
	}
	return domain.Conta{}, fmt.Errorf("conta da chave %s: %w", chave, ports.ErrNaoEncontrado)
}

func (r contas) Buscar(_ context.Context, id string) (domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.contas[id]
	if !ok {
		return c, fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return c, nil
}

func (r contas) Listar(_ context.Context) ([]domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	out := slices.Collect(maps.Values(r.s.d.contas))
	slices.SortFunc(out, func(a, b domain.Conta) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (r contas) Atualizar(_ context.Context, c domain.Conta) error {
	r.s.lock()
	defer r.s.unlock()
	atual, ok := r.s.d.contas[c.ID]
	if !ok {
		return fmt.Errorf("conta %s: %w", c.ID, ports.ErrNaoEncontrado)
	}
	c.PSP, c.Certificado = atual.PSP, atual.Certificado // Atualizar não mexe em ID, PSP e certificado
	r.s.d.contas[c.ID] = c
	return nil
}

func (r contas) AtualizarCertificado(_ context.Context, id string, cert domain.Certificado) error {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.contas[id]
	if !ok {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	// O aviso vale para a impressão digital: a mesma mantém, outra zera.
	cert.VencimentoAvisado = c.Certificado != nil && c.Certificado.VencimentoAvisado && c.Certificado.ImpressaoDigital == cert.ImpressaoDigital
	c.Certificado = &cert
	r.s.d.contas[id] = c
	return nil
}

func (r contas) MarcarVencimentoAvisado(_ context.Context, id, impressao string) error {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.contas[id]
	if !ok || c.Certificado == nil || c.Certificado.ImpressaoDigital != impressao {
		return fmt.Errorf("conta %s com o certificado %s: %w", id, impressao, ports.ErrNaoEncontrado)
	}
	cert := *c.Certificado // cópia: quem leu a conta antes não vê a mudança
	cert.VencimentoAvisado = true
	c.Certificado = &cert
	r.s.d.contas[id] = c
	return nil
}

func (r contas) SalvarCredenciais(_ context.Context, id string, cifrado []byte) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[id]; !ok {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	r.s.d.creds[id] = slices.Clone(cifrado)
	return nil
}

func (r contas) BuscarCredenciais(_ context.Context, id string) ([]byte, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.creds[id]
	if !ok {
		return nil, fmt.Errorf("credenciais da conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return slices.Clone(c), nil
}

func (r contas) SalvarTokenWebhook(_ context.Context, id string, hash []byte) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[id]; !ok {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	if hash == nil {
		delete(r.s.d.tokens, id)
		return nil
	}
	r.s.d.tokens[id] = slices.Clone(hash)
	return nil
}

func (r contas) BuscarTokenWebhook(_ context.Context, id string) ([]byte, error) {
	r.s.lock()
	defer r.s.unlock()
	h, ok := r.s.d.tokens[id]
	if !ok {
		return nil, fmt.Errorf("webhook da conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return slices.Clone(h), nil
}

type cobrancas struct{ s *Store }

func chave(contaID string, txid domain.Txid) string { return contaID + "/" + string(txid) }

func (r cobrancas) Criar(_ context.Context, c domain.Cobranca) error {
	r.s.lock()
	defer r.s.unlock()
	k := chave(c.ContaID, c.Txid)
	if r.s.d.txidUsado(c.Txid) {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrJaExiste)
	}
	c.Revisao = 0
	r.s.d.cobrancas[k] = c
	r.s.d.ordem = append(r.s.d.ordem, k)
	return nil
}

func (r cobrancas) Buscar(_ context.Context, contaID string, txid domain.Txid) (domain.Cobranca, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.cobrancas[chave(contaID, txid)]
	if !ok {
		return c, fmt.Errorf("cobrança %s: %w", txid, ports.ErrNaoEncontrado)
	}
	return c, nil
}

func (r cobrancas) BuscarPorTxid(_ context.Context, txid domain.Txid) (domain.Cobranca, error) {
	r.s.lock()
	defer r.s.unlock()
	for _, c := range r.s.d.cobrancas {
		if c.Txid == txid {
			return c, nil
		}
	}
	return domain.Cobranca{}, fmt.Errorf("cobrança %s: %w", txid, ports.ErrNaoEncontrado)
}

func (r cobrancas) Contar(_ context.Context, f ports.FiltroCobrancas) (int, error) {
	r.s.lock()
	defer r.s.unlock()
	n := 0
	for _, k := range r.s.d.ordem {
		if combina(r.s.d.cobrancas[k], f) {
			n++
		}
	}
	return n, nil
}

func (r cobrancas) Atualizar(_ context.Context, c *domain.Cobranca) error {
	r.s.lock()
	defer r.s.unlock()
	k := chave(c.ContaID, c.Txid)
	atual, ok := r.s.d.cobrancas[k]
	if !ok {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != c.Revisao {
		return fmt.Errorf("cobrança %s na revisão %d, gravando sobre %d: %w", c.Txid, atual.Revisao, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	r.s.d.cobrancas[k] = *c
	return nil
}

func (r cobrancas) Listar(_ context.Context, f ports.FiltroCobrancas) ([]domain.Cobranca, string, error) {
	r.s.lock()
	defer r.s.unlock()
	inicio := 0
	if f.Cursor != "" {
		n, err := strconv.Atoi(f.Cursor)
		if err != nil || n < 0 {
			return nil, "", fmt.Errorf("cursor inválido %q", f.Cursor)
		}
		inicio = n
	}
	limite := f.Limite
	if limite <= 0 {
		limite = 100
	}
	var out []domain.Cobranca
	pular := f.Pular
	for i := inicio; i < len(r.s.d.ordem); i++ {
		c := r.s.d.cobrancas[r.s.d.ordem[i]]
		if !combina(c, f) {
			continue
		}
		if pular > 0 {
			pular--
			continue
		}
		if len(out) == limite {
			return out, strconv.Itoa(i), nil
		}
		out = append(out, c)
	}
	return out, "", nil
}

func combina(c domain.Cobranca, f ports.FiltroCobrancas) bool {
	if f.ContaID != "" && c.ContaID != f.ContaID {
		return false
	}
	if f.Status != "" && c.Status != f.Status {
		return false
	}
	if f.Documento != "" && (c.Devedor == nil || c.Devedor.Documento != f.Documento) {
		return false
	}
	if !f.Janela.Inicio.IsZero() && c.CriadaEm.Before(f.Janela.Inicio) {
		return false
	}
	if !f.Janela.Fim.IsZero() && !c.CriadaEm.Before(f.Janela.Fim) {
		return false
	}
	return true
}

type outbox struct{ s *Store }

func (r outbox) Adicionar(_ context.Context, e domain.Evento) error {
	r.s.lock()
	defer r.s.unlock()
	if _, err := json.Marshal(e.Dados); err != nil {
		return fmt.Errorf("evento %s: serializando dados: %w", e.ID, err)
	}
	r.s.d.outbox = append(r.s.d.outbox, e)
	return nil
}

func (r outbox) NaoDistribuidos(_ context.Context, limite int) ([]ports.EventoArmazenado, error) {
	r.s.lock()
	defer r.s.unlock()
	var out []ports.EventoArmazenado
	for _, e := range r.s.d.outbox {
		if r.s.d.distrib[e.ID] {
			continue
		}
		dados, _ := json.Marshal(e.Dados)
		out = append(out, ports.EventoArmazenado{ID: e.ID, Tipo: e.Tipo, ContaID: e.ContaID, OcorridoEm: e.OcorridoEm, Dados: dados})
		if len(out) == limite {
			break
		}
	}
	return out, nil
}

func (r outbox) MarcarDistribuido(_ context.Context, id string) error {
	r.s.lock()
	defer r.s.unlock()
	r.s.d.distrib[id] = true
	return nil
}

type apikeys struct{ s *Store }

func (r apikeys) Criar(_ context.Context, k domain.APIKey) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.apikeys[k.ID]; ok {
		return fmt.Errorf("api key %s: %w", k.ID, ports.ErrJaExiste)
	}
	if k.ContaID != "" {
		if _, ok := r.s.d.contas[k.ContaID]; !ok {
			return fmt.Errorf("conta %s: %w", k.ContaID, ports.ErrNaoEncontrado)
		}
	}
	k.Escopos, k.Hash = slices.Clone(k.Escopos), slices.Clone(k.Hash)
	r.s.d.apikeys[k.ID] = k
	return nil
}

func (r apikeys) Buscar(_ context.Context, id string) (domain.APIKey, error) {
	r.s.lock()
	defer r.s.unlock()
	k, ok := r.s.d.apikeys[id]
	if !ok {
		return k, fmt.Errorf("api key %s: %w", id, ports.ErrNaoEncontrado)
	}
	return k, nil
}

func (r apikeys) Listar(_ context.Context) ([]domain.APIKey, error) {
	r.s.lock()
	defer r.s.unlock()
	out := slices.Collect(maps.Values(r.s.d.apikeys))
	slices.SortFunc(out, func(a, b domain.APIKey) int { return a.CriadaEm.Compare(b.CriadaEm) })
	return out, nil
}

func (r apikeys) Revogar(_ context.Context, id string, quando time.Time) error {
	r.s.lock()
	defer r.s.unlock()
	k, ok := r.s.d.apikeys[id]
	if !ok {
		return fmt.Errorf("api key %s: %w", id, ports.ErrNaoEncontrado)
	}
	if k.RevogadaEm == nil {
		k.RevogadaEm = &quando
		r.s.d.apikeys[id] = k
	}
	return nil
}

type idem struct{ s *Store }

func (r idem) Buscar(_ context.Context, dono, chave string) (domain.RegistroIdempotencia, error) {
	r.s.lock()
	defer r.s.unlock()
	reg, ok := r.s.d.idem[[2]string{dono, chave}]
	if !ok {
		return reg, fmt.Errorf("idempotência %s: %w", chave, ports.ErrNaoEncontrado)
	}
	return reg, nil
}

func (r idem) Salvar(_ context.Context, reg domain.RegistroIdempotencia) error {
	r.s.lock()
	defer r.s.unlock()
	reg.Hash, reg.Corpo = slices.Clone(reg.Hash), slices.Clone(reg.Corpo)
	r.s.d.idem[[2]string{reg.Dono, reg.Chave}] = reg
	return nil
}

func (r idem) Remover(_ context.Context, dono, chave string) error {
	r.s.lock()
	defer r.s.unlock()
	delete(r.s.d.idem, [2]string{dono, chave})
	return nil
}

// txidUsado confere cobs, cobvs e cobrs: o txid é único na instância (RN-10).
func (d *dados) txidUsado(txid domain.Txid) bool {
	if _, ok := d.cobvs[txid]; ok {
		return true
	}
	if _, ok := d.cobrs[txid]; ok {
		return true
	}
	for _, c := range d.cobrancas {
		if c.Txid == txid {
			return true
		}
	}
	return false
}

type cobvs struct{ s *Store }

func (r cobvs) Criar(_ context.Context, c domain.CobrancaVencimento) error {
	r.s.lock()
	defer r.s.unlock()
	if r.s.d.txidUsado(c.Txid) {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrJaExiste)
	}
	if _, ok := r.s.d.contas[c.ContaID]; !ok {
		return fmt.Errorf("conta %s: %w", c.ContaID, ports.ErrNaoEncontrado)
	}
	c.Revisao = 0
	r.s.d.cobvs[c.Txid] = c
	r.s.d.ordemV = append(r.s.d.ordemV, c.Txid)
	return nil
}

func (r cobvs) BuscarPorTxid(_ context.Context, txid domain.Txid) (domain.CobrancaVencimento, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.cobvs[txid]
	if !ok {
		return c, fmt.Errorf("cobrança %s: %w", txid, ports.ErrNaoEncontrado)
	}
	return c, nil
}

func (r cobvs) Atualizar(_ context.Context, c *domain.CobrancaVencimento) error {
	r.s.lock()
	defer r.s.unlock()
	atual, ok := r.s.d.cobvs[c.Txid]
	if !ok {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != c.Revisao {
		return fmt.Errorf("cobrança %s na revisão %d, gravando sobre %d: %w", c.Txid, atual.Revisao, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	r.s.d.cobvs[c.Txid] = *c
	return nil
}

func (r cobvs) filtrar(f ports.FiltroCobrancas) []domain.CobrancaVencimento {
	var out []domain.CobrancaVencimento
	for _, tx := range r.s.d.ordemV {
		c := r.s.d.cobvs[tx]
		casa := combina(domain.Cobranca{ContaID: c.ContaID, Status: c.Status, CriadaEm: c.CriadaEm, Devedor: &c.Devedor.Devedor}, f)
		if casa {
			out = append(out, c)
		}
	}
	return out
}

func (r cobvs) Listar(_ context.Context, f ports.FiltroCobrancas) ([]domain.CobrancaVencimento, string, error) {
	r.s.lock()
	defer r.s.unlock()
	todas := r.filtrar(f)
	inicio := 0
	if f.Cursor != "" {
		n, err := strconv.Atoi(f.Cursor)
		if err != nil || n < 0 {
			return nil, "", fmt.Errorf("cursor inválido %q", f.Cursor)
		}
		inicio = n
	}
	inicio += f.Pular
	if inicio >= len(todas) {
		return nil, "", nil
	}
	limite := f.Limite
	if limite <= 0 {
		limite = 100
	}
	fim := min(inicio+limite, len(todas))
	prox := ""
	if fim < len(todas) {
		prox = strconv.Itoa(fim)
	}
	return slices.Clone(todas[inicio:fim]), prox, nil
}

func (r cobvs) Contar(_ context.Context, f ports.FiltroCobrancas) (int, error) {
	r.s.lock()
	defer r.s.unlock()
	return len(r.filtrar(f)), nil
}

type pixes struct{ s *Store }

func (r pixes) Registrar(_ context.Context, p domain.PixRecebido) (bool, error) {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.pix[p.EndToEndID]; ok {
		return false, nil
	}
	if _, ok := r.s.d.contas[p.ContaID]; !ok {
		return false, fmt.Errorf("conta %s: %w", p.ContaID, ports.ErrNaoEncontrado)
	}
	r.s.d.pix[p.EndToEndID] = p
	return true, nil
}

func (r pixes) BuscarPorE2E(_ context.Context, e2e string) (domain.PixRecebido, error) {
	r.s.lock()
	defer r.s.unlock()
	p, ok := r.s.d.pix[e2e]
	if !ok {
		return p, fmt.Errorf("pix %s: %w", e2e, ports.ErrNaoEncontrado)
	}
	return p, nil
}

func (r pixes) ListarPorTxid(_ context.Context, txid domain.Txid) ([]domain.PixRecebido, error) {
	r.s.lock()
	defer r.s.unlock()
	var out []domain.PixRecebido
	for _, p := range r.s.d.pix {
		if p.Txid == txid {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b domain.PixRecebido) int { return a.Horario.Compare(b.Horario) })
	return out, nil
}

type destinos struct{ s *Store }

func (r destinos) Criar(_ context.Context, d domain.Destino) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.destinos[d.ID]; ok {
		return fmt.Errorf("destino %s: %w", d.ID, ports.ErrJaExiste)
	}
	if d.ContaID != "" {
		if _, ok := r.s.d.contas[d.ContaID]; !ok {
			return fmt.Errorf("conta %s: %w", d.ContaID, ports.ErrNaoEncontrado)
		}
	}
	r.s.d.destinos[d.ID] = d
	return nil
}

func (r destinos) AtualizarSegredo(_ context.Context, id string, cifrado []byte) error {
	r.s.lock()
	defer r.s.unlock()
	d, ok := r.s.d.destinos[id]
	if !ok {
		return fmt.Errorf("destino %s: %w", id, ports.ErrNaoEncontrado)
	}
	d.SegredoCifrado = slices.Clone(cifrado)
	r.s.d.destinos[id] = d
	return nil
}

func (r destinos) Listar(_ context.Context) ([]domain.Destino, error) {
	r.s.lock()
	defer r.s.unlock()
	out := slices.Collect(maps.Values(r.s.d.destinos))
	slices.SortFunc(out, func(a, b domain.Destino) int { return a.CriadoEm.Compare(b.CriadoEm) })
	return out, nil
}

func (r destinos) DaConta(ctx context.Context, contaID string) ([]domain.Destino, error) {
	todos, _ := r.Listar(ctx)
	var out []domain.Destino
	for _, d := range todos {
		if d.ContaID == "" || d.ContaID == contaID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (r destinos) Remover(_ context.Context, id string) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.destinos[id]; !ok {
		return fmt.Errorf("destino %s: %w", id, ports.ErrNaoEncontrado)
	}
	delete(r.s.d.destinos, id)
	return nil
}

type entregas struct{ s *Store }

func (r entregas) Criar(_ context.Context, e domain.Entrega) error {
	r.s.lock()
	defer r.s.unlock()
	k := [2]string{e.EventoID, e.DestinoID}
	if _, ok := r.s.d.entregas[k]; !ok {
		e.Corpo = slices.Clone(e.Corpo)
		r.s.d.entregas[k] = e
	}
	return nil
}

func (r entregas) ordenadas(filtro func(domain.Entrega) bool, limite int) []domain.Entrega {
	var out []domain.Entrega
	for _, e := range r.s.d.entregas {
		if filtro(e) {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b domain.Entrega) int {
		if c := a.ProximaEm.Compare(b.ProximaEm); c != 0 {
			return c
		}
		return cmp.Compare(a.EventoID+a.DestinoID, b.EventoID+b.DestinoID)
	})
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

func (r entregas) Proximas(_ context.Context, agora time.Time, limite int) ([]domain.Entrega, error) {
	r.s.lock()
	defer r.s.unlock()
	return r.ordenadas(func(e domain.Entrega) bool { return e.Status == domain.EntregaPendente && !e.ProximaEm.After(agora) }, limite), nil
}

func (r entregas) Atualizar(_ context.Context, e domain.Entrega) error {
	r.s.lock()
	defer r.s.unlock()
	k := [2]string{e.EventoID, e.DestinoID}
	if _, ok := r.s.d.entregas[k]; !ok {
		return fmt.Errorf("entrega %s/%s: %w", e.EventoID, e.DestinoID, ports.ErrNaoEncontrado)
	}
	r.s.d.entregas[k] = e
	return nil
}

func (r entregas) Listar(_ context.Context, status domain.StatusEntrega, limite int) ([]domain.Entrega, error) {
	r.s.lock()
	defer r.s.unlock()
	return r.ordenadas(func(e domain.Entrega) bool { return e.Status == status }, limite), nil
}

func (r entregas) Reenviar(_ context.Context, eventoID string, agora time.Time) (int, error) {
	r.s.lock()
	defer r.s.unlock()
	n := 0
	for k, e := range r.s.d.entregas {
		if e.EventoID == eventoID && e.Status == domain.EntregaDLQ {
			e.Status, e.Tentativas, e.ProximaEm, e.AtualizadaEm = domain.EntregaPendente, 0, agora, agora
			r.s.d.entregas[k] = e
			n++
		}
	}
	return n, nil
}
