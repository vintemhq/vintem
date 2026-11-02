package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// Webhooks configura a URL de aviso no banco e recebe os avisos (RN-30).
type Webhooks struct {
	Store        ports.Store
	PSPs         ports.Registry
	Recebimentos Recebimentos
	// Recorrencias aplica os avisos de Pix Automático (webhookrec).
	Recorrencias Recorrencias
	// CobrancasRecorrentes aplica os avisos de cobrança recorrente (webhookcobr).
	CobrancasRecorrentes CobrancasRecorrentes
}

// Configurar gera um token novo, registra a URL no PSP e só então grava o
// token: se o PSP recusar, a URL antiga continua valendo.
func (s Webhooks) Configurar(ctx context.Context, contaID, urlPublica string) (string, error) {
	base, err := url.Parse(strings.TrimRight(urlPublica, "/"))
	local := err == nil && (base.Hostname() == "localhost" || base.Hostname() == "127.0.0.1")
	esquemaOK := err == nil && (base.Scheme == "https" || (base.Scheme == "http" && local))
	if err != nil || base.Host == "" || !esquemaOK {
		return "", &ErroValidacao{Violacoes: []Violacao{{Propriedade: "urlPublica", Razao: "use https:// (http só para localhost)", Valor: urlPublica}}}
	}
	conta, err := s.Store.Contas().Buscar(ctx, contaID)
	if err != nil {
		return "", err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapWebhook)
	if err != nil {
		return "", err
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	endereco := base.String() + "/webhooks/pix/" + conta.ID + "/" + token
	// O token é gravado antes de falar com o banco: o Efí chama a URL durante o
	// registro e só aceita se ela responder 2xx (produção, 30/10/2026). Se o banco
	// recusar, o token anterior volta, porque a URL antiga continua valendo lá.
	anterior, errAnterior := s.Store.Contas().BuscarTokenWebhook(ctx, conta.ID)
	if errAnterior != nil && !errors.Is(errAnterior, ports.ErrNaoEncontrado) {
		return "", errAnterior
	}
	h := sha256.Sum256([]byte(token))
	if err := s.Store.Contas().SalvarTokenWebhook(ctx, conta.ID, h[:]); err != nil {
		return "", fmt.Errorf("gravando o token do webhook: %w", err)
	}
	if err := psp.ConfigurarWebhook(ctx, conta, endereco); err != nil {
		if errAnterior == nil {
			if errVolta := s.Store.Contas().SalvarTokenWebhook(ctx, conta.ID, anterior); errVolta != nil {
				return "", fmt.Errorf("o banco recusou a URL nova e o token anterior não voltou; configure de novo: %w", errors.Join(erroDoPSP(err), errVolta))
			}
		}
		return "", erroDoPSP(err)
	}
	// Com Pix Automático, o mesmo token vale para o aviso de recorrência. Fica
	// depois de gravar o token: se falhar, o aviso de Pix continua valendo.
	if rec, err := pspRecorrencia(s.PSPs, conta); err == nil {
		if err := rec.ConfigurarWebhookRec(ctx, conta, base.String()+"/webhooks/rec/"+conta.ID+"/"+token); err != nil {
			return endereco, fmt.Errorf("o aviso de Pix foi configurado, mas o de recorrência não; configure de novo: %w", erroDoPSP(err))
		}
	}
	// O aviso de cobrança recorrente tem URL própria (webhookcobr), com o mesmo token.
	if cobr, err := pspCobr(s.PSPs, conta); err == nil {
		if err := cobr.ConfigurarWebhookCobr(ctx, conta, base.String()+"/webhooks/cobr/"+conta.ID+"/"+token); err != nil {
			return endereco, fmt.Errorf("o aviso de Pix foi configurado, mas o de cobrança recorrente não; configure de novo: %w", erroDoPSP(err))
		}
	}
	return endereco, nil
}

// Remover cancela o aviso de Pix registrado no banco e só então apaga o token
// local, o que invalida as URLs de Pix, recorrência e cobrança recorrente da
// conta. Se o banco falhar, o token continua valendo: a URL ainda está
// registrada lá, e os avisos seguem chegando. Banco sem webhook na chave conta
// como já removido, e o token é apagado do mesmo jeito; registrado diz se havia
// algo para cancelar no banco.
func (s Webhooks) Remover(ctx context.Context, contaID string) (registrado bool, err error) {
	conta, err := s.Store.Contas().Buscar(ctx, contaID)
	if err != nil {
		return false, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapWebhook)
	if err != nil {
		return false, err
	}
	registrado = true
	if err := psp.RemoverWebhook(ctx, conta); errors.Is(err, ports.ErrNaoEncontrado) {
		registrado = false
	} else if err != nil {
		return false, erroDoPSP(err)
	}
	if err := s.Store.Contas().SalvarTokenWebhook(ctx, conta.ID, nil); err != nil {
		return registrado, fmt.Errorf("o webhook saiu do banco, mas o token local não foi apagado; rode de novo: %w", err)
	}
	return registrado, nil
}

// contaDoToken confere o token da URL do webhook e devolve a conta. Token
// ausente ou errado é ErrNaoAutenticado, sem dizer qual dos dois.
func (s Webhooks) contaDoToken(ctx context.Context, contaID, token string) (domain.Conta, error) {
	esperado, err := s.Store.Contas().BuscarTokenWebhook(ctx, contaID)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return domain.Conta{}, domain.ErrTokenWebhook
	}
	if err != nil {
		return domain.Conta{}, err
	}
	h := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(h[:], esperado) != 1 {
		return domain.Conta{}, domain.ErrTokenWebhook
	}
	return s.Store.Contas().Buscar(ctx, contaID)
}

// ReceberRec confere o token, interpreta o aviso de recorrência pelo adapter
// da conta e aplica cada mudança. Devolve quantas recorrências mudaram.
func (s Webhooks) ReceberRec(ctx context.Context, contaID, token string, corpo []byte) (int, error) {
	conta, err := s.contaDoToken(ctx, contaID, token)
	if err != nil {
		return 0, err
	}
	psp, err := pspRecorrencia(s.PSPs, conta)
	if err != nil {
		return 0, err
	}
	ats, err := psp.InterpretarWebhookRec(conta, corpo)
	if err != nil {
		return 0, err
	}
	mudaram := 0
	for _, a := range ats {
		mudou, err := s.Recorrencias.RegistrarAtualizacao(ctx, conta.ID, a)
		if errors.Is(err, ports.ErrNaoEncontrado) {
			// Recorrência criada fora do Vintém, ou de outra conta: responder erro
			// faria o banco reenviar para sempre.
			slog.WarnContext(ctx, "aviso de recorrência desconhecida ignorado", "conta", conta.ID, "idRec", a.IDRec)
			continue
		}
		if err != nil {
			return mudaram, err
		}
		if mudou {
			mudaram++
		}
	}
	return mudaram, nil
}

// ReceberCobr confere o token, interpreta o aviso de cobrança recorrente pelo
// adapter da conta e aplica cada mudança. Devolve quantas cobranças mudaram.
func (s Webhooks) ReceberCobr(ctx context.Context, contaID, token string, corpo []byte) (int, error) {
	conta, err := s.contaDoToken(ctx, contaID, token)
	if err != nil {
		return 0, err
	}
	psp, err := pspCobr(s.PSPs, conta)
	if err != nil {
		return 0, err
	}
	ats, err := psp.InterpretarWebhookCobr(conta, corpo)
	if err != nil {
		return 0, err
	}
	mudaram := 0
	for _, a := range ats {
		mudou, err := s.CobrancasRecorrentes.RegistrarAtualizacao(ctx, conta.ID, a)
		if errors.Is(err, ports.ErrNaoEncontrado) {
			// Cobrança criada fora do Vintém, ou de outra conta: responder erro
			// faria o banco reenviar para sempre.
			slog.WarnContext(ctx, "aviso de cobrança recorrente desconhecida ignorado", "conta", conta.ID, "txid", a.Txid)
			continue
		}
		if err != nil {
			return mudaram, err
		}
		if mudou {
			mudaram++
		}
	}
	return mudaram, nil
}

// Receber confere o token da URL, interpreta o corpo pelo adapter da conta e
// registra os Pix. Só devolve sucesso depois de tudo gravado (RN-30).
func (s Webhooks) Receber(ctx context.Context, contaID, token string, corpo []byte) (int, error) {
	conta, err := s.contaDoToken(ctx, contaID, token)
	if err != nil {
		return 0, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapWebhook)
	if err != nil {
		return 0, err
	}
	pixes, err := psp.InterpretarWebhook(conta, corpo)
	if err != nil {
		return 0, err
	}
	for i := range pixes {
		pixes[i].ContaID = conta.ID // o adapter não decide a conta: a URL decide
	}
	return s.Recebimentos.Registrar(ctx, pixes)
}
