// Package ports define as interfaces que o núcleo exige do mundo externo:
// PSP, Store, Notifier, Vault e Clock. Nada aqui conhece um banco específico.
package ports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
)

// ErrNaoSuportado indica que o PSP da conta não oferece a capacidade pedida (RN-50).
var ErrNaoSuportado = errors.New("capacidade não suportada pelo PSP")

// Capacidade é algo que um PSP pode ou não oferecer.
type Capacidade string

// Capacidades conhecidas (RN-50).
const (
	CapCob       Capacidade = "cob"
	CapCobv      Capacidade = "cobv"
	CapDevolucao Capacidade = "devolucao"
	CapWebhook   Capacidade = "webhook"
	CapRec       Capacidade = "rec"
	CapCobr      Capacidade = "cobr"
)

// Capabilities é o conjunto de capacidades declaradas por um adapter.
type Capabilities map[Capacidade]bool

// Suporta devolve ErrNaoSuportado, com o nome da capacidade, se ela faltar.
func (c Capabilities) Suporta(capacidade Capacidade) error {
	if !c[capacidade] {
		return fmt.Errorf("%w: %s", ErrNaoSuportado, capacidade)
	}
	return nil
}

// Janela é um intervalo de tempo [Inicio, Fim) para consultas como GET /pix.
type Janela struct {
	Inicio, Fim time.Time
}

// PSP é o adapter de um banco ou provedor Pix. Todo desvio do banco em relação
// à spec do BCB é tratado dentro do adapter (RN-51).
type PSP interface {
	// Nome é o identificador do adapter no registry ("efi", "inter", "sandbox"...).
	Nome() string
	Capabilities() Capabilities

	CriarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error)
	ConsultarCob(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error)
	// RevisarCob envia o novo estado da cobrança (valor, devedor, expiração,
	// solicitação ou remoção pelo recebedor) e devolve o que o PSP gravou.
	RevisarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error)
	// Cobrança com vencimento: só se Capabilities()[CapCobv].
	CriarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error)
	ConsultarCobv(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaVencimento, error)
	RevisarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error)

	ListarPix(ctx context.Context, conta domain.Conta, j Janela) ([]domain.PixRecebido, error)
	Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error)
	// ConsultarDevolucao devolve a devolução como o banco a conhece (GET
	// /pix/{e2eid}/devolucao/{id}); inexistente é ErrNaoEncontrado.
	ConsultarDevolucao(ctx context.Context, conta domain.Conta, e2e, id string) (domain.Devolucao, error)
	ConfigurarWebhook(ctx context.Context, conta domain.Conta, url string) error
	// RemoverWebhook cancela o aviso de Pix registrado na chave da conta
	// (DELETE /webhook/{chave}). Sem webhook registrado, ErrNaoEncontrado.
	RemoverWebhook(ctx context.Context, conta domain.Conta) error
	// InterpretarWebhook lê o corpo que o PSP enviou ao webhook. Quem segue a
	// spec usa psp.WebhookPixBCB; quem desvia trata o desvio aqui.
	InterpretarWebhook(conta domain.Conta, corpo []byte) ([]domain.PixRecebido, error)
}

// PSPRecorrencia é a parte opcional do PSP para Pix Automático (F6). Só o
// adapter que declara CapRec a implementa; o caso de uso confere as duas coisas.
type PSPRecorrencia interface {
	// CriarRecorrencia envia a recorrência (POST /rec) e devolve o que o PSP
	// gravou, com o idRec gerado por ele e o status CRIADA.
	CriarRecorrencia(ctx context.Context, conta domain.Conta, r domain.Recorrencia) (domain.Recorrencia, error)
	ConsultarRecorrencia(ctx context.Context, conta domain.Conta, id string) (domain.Recorrencia, error)
	// SolicitarConfirmacao pede a autorização no app do pagador (POST
	// /solicrec, jornada 1) e devolve a solicitação com o idSolicRec do PSP.
	SolicitarConfirmacao(ctx context.Context, conta domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error)
	// CancelarSolicitacao desiste do pedido antes da resposta do pagador (PATCH
	// /solicrec/{idSolicRec} com CANCELADA).
	CancelarSolicitacao(ctx context.Context, conta domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error)
	// ConfigurarWebhookRec registra a URL do webhook de recorrência (PUT /webhookrec).
	ConfigurarWebhookRec(ctx context.Context, conta domain.Conta, url string) error
	// InterpretarWebhookRec lê o aviso de recorrência que o PSP enviou. Quem
	// segue a spec usa psp.WebhookRecBCB.
	InterpretarWebhookRec(conta domain.Conta, corpo []byte) ([]domain.AtualizacaoRecorrencia, error)
}

// PSPCobrancaRecorrente é a parte opcional do PSP para a cobrança recorrente
// do Pix Automático (RN-42). Só o adapter que declara CapCobr a implementa.
type PSPCobrancaRecorrente interface {
	// CriarCobr envia a cobrança com o txid dela (PUT /cobr/{txid}) e devolve
	// o que o PSP gravou.
	CriarCobr(ctx context.Context, conta domain.Conta, c domain.CobrancaRecorrente) (domain.CobrancaRecorrente, error)
	ConsultarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaRecorrente, error)
	// CancelarCobr pede o cancelamento (PATCH /cobr/{txid} com CANCELADA).
	CancelarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaRecorrente, error)
	// SolicitarRetentativa pede uma nova data de liquidação (POST
	// /cobr/{txid}/retentativa/{data}) e devolve a cobrança com a tentativa nova.
	SolicitarRetentativa(ctx context.Context, conta domain.Conta, txid domain.Txid, data domain.Data) (domain.CobrancaRecorrente, error)
	// ConfigurarWebhookCobr registra a URL do webhook de cobrança recorrente (PUT /webhookcobr).
	ConfigurarWebhookCobr(ctx context.Context, conta domain.Conta, url string) error
	// InterpretarWebhookCobr lê o aviso de cobrança recorrente que o PSP
	// enviou. Quem segue a spec usa psp.WebhookCobrBCB.
	InterpretarWebhookCobr(conta domain.Conta, corpo []byte) ([]domain.AtualizacaoCobrancaRecorrente, error)
}

// Credenciais entrega a um adapter de PSP as credenciais decifradas de uma conta
// (RN-61). Só o adapter pede; nada mais no núcleo vê o segredo em claro.
type Credenciais interface {
	Credenciais(ctx context.Context, contaID string) (domain.Credenciais, error)
}

// ErrPSPDesconhecido indica um nome de PSP que nenhum adapter registrou.
var ErrPSPDesconhecido = errors.New("psp desconhecido")

// Registry encontra o adapter de PSP pelo nome gravado na conta.
type Registry interface {
	PSP(nome string) (PSP, error)
}
