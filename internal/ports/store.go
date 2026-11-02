package ports

import (
	"context"
	"errors"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
)

// Erros comuns a todo Store.
var (
	ErrNaoEncontrado = errors.New("não encontrado")
	ErrJaExiste      = errors.New("já existe")
	// ErrConflito indica que o registro mudou desde a leitura (revisão desatualizada).
	ErrConflito = errors.New("conflito de revisão")
)

// Store dá acesso aos repositórios e à transação.
type Store interface {
	Contas() ContaRepo
	Cobrancas() CobrancaRepo
	CobrancasV() CobrancaVRepo
	Pix() PixRepo
	Outbox() OutboxRepo
	APIKeys() APIKeyRepo
	Idempotencia() IdempotenciaRepo
	Destinos() DestinoRepo
	Entregas() EntregaRepo
	Recorrencias() RecorrenciaRepo
	Solicitacoes() SolicitacaoRepo
	CobrancasRecorrentes() CobrancaRecorrenteRepo
	Devolucoes() DevolucaoRepo
	// WithTx executa fn numa transação: erro em fn desfaz tudo.
	WithTx(ctx context.Context, fn func(Store) error) error
}

// RecorrenciaRepo guarda as recorrências de Pix Automático (RN-40). O idRec,
// gerado pelo PSP, é único na instância.
type RecorrenciaRepo interface {
	// Criar falha com ErrJaExiste se o idRec já existir.
	Criar(ctx context.Context, r domain.Recorrencia) error
	Buscar(ctx context.Context, id string) (domain.Recorrencia, error)
	// Atualizar grava status e motivo de rejeição se r.Revisao for a revisão
	// atual, e a incrementa; senão falha com ErrConflito.
	Atualizar(ctx context.Context, r *domain.Recorrencia) error
	// Abertas devolve as recorrências da conta que ainda podem mudar (CRIADA e
	// APROVADA), em ordem de criação, para a conciliação.
	Abertas(ctx context.Context, contaID string) ([]domain.Recorrencia, error)
}

// SolicitacaoRepo guarda as solicitações de confirmação de recorrência (RN-41).
type SolicitacaoRepo interface {
	// Criar falha com ErrJaExiste se o idSolicRec já existir, e com
	// ErrNaoEncontrado se a recorrência não existir.
	Criar(ctx context.Context, s domain.SolicitacaoRecorrencia) error
	Buscar(ctx context.Context, id string) (domain.SolicitacaoRecorrencia, error)
	// AtualizarStatus grava o novo status. ErrNaoEncontrado se não existir.
	AtualizarStatus(ctx context.Context, id string, status domain.StatusSolicitacao) error
	// DaRecorrencia devolve as solicitações da recorrência, em ordem de criação.
	DaRecorrencia(ctx context.Context, idRec string) ([]domain.SolicitacaoRecorrencia, error)
}

// CobrancaRecorrenteRepo guarda as cobranças recorrentes (RN-42). O txid é
// único na instância, compartilhado com cob e cobv (RN-10).
type CobrancaRecorrenteRepo interface {
	// Criar falha com ErrJaExiste se o txid já existir, e com
	// ErrNaoEncontrado se a recorrência não existir.
	Criar(ctx context.Context, c domain.CobrancaRecorrente) error
	BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.CobrancaRecorrente, error)
	// Atualizar grava status, tentativas e motivo de encerramento se
	// c.Revisao for a revisão atual, e a incrementa; senão falha com ErrConflito.
	Atualizar(ctx context.Context, c *domain.CobrancaRecorrente) error
	// DaRecorrencia devolve as cobranças da recorrência em ordem de vencimento.
	DaRecorrencia(ctx context.Context, idRec string) ([]domain.CobrancaRecorrente, error)
	// Abertas devolve as cobranças da conta que ainda podem mudar (CRIADA e
	// ATIVA), em ordem de criação, para a conciliação.
	Abertas(ctx context.Context, contaID string) ([]domain.CobrancaRecorrente, error)
}

// ContaRepo guarda as contas recebedoras e as credenciais delas, já cifradas.
type ContaRepo interface {
	Criar(ctx context.Context, c domain.Conta) error
	Buscar(ctx context.Context, id string) (domain.Conta, error)
	// BuscarPorChave encontra a conta dona da chave Pix (chave é única na instância).
	BuscarPorChave(ctx context.Context, chave string) (domain.Conta, error)
	Listar(ctx context.Context) ([]domain.Conta, error)
	// Atualizar grava nome, chave, ambiente e ativa. ID e PSP não mudam.
	Atualizar(ctx context.Context, c domain.Conta) error
	// SalvarCredenciais grava o blob cifrado pelo Vault; o Store nunca vê o segredo em claro.
	SalvarCredenciais(ctx context.Context, contaID string, cifrado []byte) error
	// AtualizarCertificado grava os dados públicos do certificado da conta.
	// Trocar a impressão digital zera o aviso de vencimento. ErrNaoEncontrado se
	// a conta não existir.
	AtualizarCertificado(ctx context.Context, contaID string, c domain.Certificado) error
	// MarcarVencimentoAvisado registra que o vencimento do certificado com essa
	// impressão digital já foi avisado. ErrNaoEncontrado se a conta não existir ou
	// tiver outro certificado.
	MarcarVencimentoAvisado(ctx context.Context, contaID, impressaoDigital string) error
	// BuscarCredenciais devolve o blob cifrado, ou ErrNaoEncontrado se a conta não tiver credenciais.
	BuscarCredenciais(ctx context.Context, contaID string) ([]byte, error)
	// SalvarTokenWebhook grava o hash do token da URL de webhook da conta. Hash
	// nil apaga o token: o BuscarTokenWebhook volta a dar ErrNaoEncontrado.
	SalvarTokenWebhook(ctx context.Context, contaID string, hash []byte) error
	// BuscarTokenWebhook devolve o hash, ou ErrNaoEncontrado se a conta não tiver webhook configurado.
	BuscarTokenWebhook(ctx context.Context, contaID string) ([]byte, error)
}

// FiltroCobrancas restringe a listagem; zero vale "sem filtro".
type FiltroCobrancas struct {
	ContaID string
	Status  domain.StatusCobranca
	// Documento filtra pelo CPF/CNPJ do devedor.
	Documento string
	Janela    Janela
	Limite    int
	// Pular descarta os primeiros N resultados (paginação por página da spec do BCB).
	// Com Cursor, prefira deixar zero.
	Pular int
	// Cursor é o valor devolvido pela página anterior; vazio começa do início.
	Cursor string
}

// CobrancaRepo guarda cobranças imediatas. O txid é único na instância (RN-10).
type CobrancaRepo interface {
	// Criar falha com ErrJaExiste se o txid já existir.
	Criar(ctx context.Context, c domain.Cobranca) error
	Buscar(ctx context.Context, contaID string, txid domain.Txid) (domain.Cobranca, error)
	// BuscarPorTxid encontra a cobrança sem saber a conta.
	BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.Cobranca, error)
	// Contar devolve quantas cobranças casam com o filtro, ignorando Limite, Pular e Cursor.
	Contar(ctx context.Context, f FiltroCobrancas) (int, error)
	// Atualizar grava se c.Revisao for a revisão atual, e a incrementa;
	// senão falha com ErrConflito.
	Atualizar(ctx context.Context, c *domain.Cobranca) error
	// Listar devolve uma página e o cursor da próxima (vazio se acabou).
	Listar(ctx context.Context, f FiltroCobrancas) ([]domain.Cobranca, string, error)
}

// OutboxRepo guarda eventos a entregar, na mesma transação da mudança de estado (ADR-005).
type OutboxRepo interface {
	Adicionar(ctx context.Context, e domain.Evento) error
	// NaoDistribuidos devolve, em ordem de criação, eventos ainda sem entregas criadas.
	NaoDistribuidos(ctx context.Context, limite int) ([]EventoArmazenado, error)
	// MarcarDistribuido registra que as entregas do evento já foram criadas.
	MarcarDistribuido(ctx context.Context, eventoID string) error
}

// EventoArmazenado é o evento como saiu do outbox, com os dados já em JSON.
type EventoArmazenado struct {
	ID         string
	Tipo       domain.TipoEvento
	ContaID    string
	OcorridoEm time.Time
	Dados      []byte
}

// APIKeyRepo guarda as API keys. Só o hash do segredo é gravado (RN-60).
type APIKeyRepo interface {
	Criar(ctx context.Context, k domain.APIKey) error
	Buscar(ctx context.Context, id string) (domain.APIKey, error)
	Listar(ctx context.Context) ([]domain.APIKey, error)
	// Revogar marca a chave como revogada em quando; revogar de novo não muda a data.
	Revogar(ctx context.Context, id string, quando time.Time) error
}

// IdempotenciaRepo guarda as respostas por (dono, chave) para o Idempotency-Key (RN-11).
type IdempotenciaRepo interface {
	// Buscar devolve o registro, ou ErrNaoEncontrado.
	Buscar(ctx context.Context, dono, chave string) (domain.RegistroIdempotencia, error)
	// Salvar grava ou substitui o registro de (dono, chave).
	Salvar(ctx context.Context, r domain.RegistroIdempotencia) error
	// Remover apaga o registro; apagar o que não existe não é erro.
	Remover(ctx context.Context, dono, chave string) error
}

// CobrancaVRepo guarda cobranças com vencimento. O txid é único na instância,
// compartilhado com as cobs: um txid usado numa cob não serve para uma cobv (RN-10).
type CobrancaVRepo interface {
	Criar(ctx context.Context, c domain.CobrancaVencimento) error
	BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.CobrancaVencimento, error)
	// Atualizar segue a mesma revisão otimista de CobrancaRepo.Atualizar.
	Atualizar(ctx context.Context, c *domain.CobrancaVencimento) error
	// Listar usa os mesmos filtros e paginação da cob; Janela filtra pela criação.
	Listar(ctx context.Context, f FiltroCobrancas) ([]domain.CobrancaVencimento, string, error)
	Contar(ctx context.Context, f FiltroCobrancas) (int, error)
}

// PixRepo guarda os Pix recebidos. O endToEndId é único: o mesmo pagamento
// avisado duas vezes (webhook repetido, conciliação) é gravado uma vez só (RN-30).
type PixRepo interface {
	// Registrar grava o Pix e devolve novo=false, sem erro, se ele já existia.
	Registrar(ctx context.Context, p domain.PixRecebido) (novo bool, err error)
	BuscarPorE2E(ctx context.Context, e2e string) (domain.PixRecebido, error)
	// ListarPorTxid devolve os Pix de uma cobrança, em ordem de horário.
	ListarPorTxid(ctx context.Context, txid domain.Txid) ([]domain.PixRecebido, error)
}

// DevolucaoRepo guarda as devoluções de Pix recebidos (RN-20, RN-21). O par
// endToEndId + id é único.
type DevolucaoRepo interface {
	// Criar falha com ErrJaExiste se o par já existir e com ErrNaoEncontrado
	// se o Pix não estiver gravado.
	Criar(ctx context.Context, d domain.Devolucao) error
	Buscar(ctx context.Context, e2e, id string) (domain.Devolucao, error)
	// DoPix devolve as devoluções do Pix em ordem de criação.
	DoPix(ctx context.Context, e2e string) ([]domain.Devolucao, error)
	// Atualizar grava status, motivo, rtrId e liquidação se a revisão bater;
	// senão, ErrConflito. Em sucesso, incrementa d.Revisao.
	Atualizar(ctx context.Context, d *domain.Devolucao) error
	// EmProcessamento devolve as devoluções da conta ainda sem resultado, em
	// ordem de criação.
	EmProcessamento(ctx context.Context, contaID string) ([]domain.Devolucao, error)
}

// DestinoRepo guarda os destinos de webhook de saída.
type DestinoRepo interface {
	Criar(ctx context.Context, d domain.Destino) error
	Listar(ctx context.Context) ([]domain.Destino, error)
	// DaConta devolve os destinos da conta e os globais (ContaID vazio).
	DaConta(ctx context.Context, contaID string) ([]domain.Destino, error)
	// AtualizarSegredo grava o segredo recifrado. ErrNaoEncontrado se não existir.
	AtualizarSegredo(ctx context.Context, id string, cifrado []byte) error
	Remover(ctx context.Context, id string) error
}

// EntregaRepo guarda as entregas de eventos a destinos (RN-32).
type EntregaRepo interface {
	// Criar ignora a entrega que já existe para (evento, destino).
	Criar(ctx context.Context, e domain.Entrega) error
	// Proximas devolve entregas pendentes com ProximaEm até agora, das mais antigas.
	Proximas(ctx context.Context, agora time.Time, limite int) ([]domain.Entrega, error)
	Atualizar(ctx context.Context, e domain.Entrega) error
	Listar(ctx context.Context, status domain.StatusEntrega, limite int) ([]domain.Entrega, error)
	// Reenviar devolve à fila as entregas do evento que estão na DLQ, zerando as tentativas.
	Reenviar(ctx context.Context, eventoID string, agora time.Time) (int, error)
}
