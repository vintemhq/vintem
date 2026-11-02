package conformance_test

import (
	"context"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
)

// soCob é um adapter que só declara cob, como um banco recém-contribuído:
// o resto responde ErrNaoSuportado.
type soCob struct{ *sandbox.PSP }

func (soCob) Capabilities() ports.Capabilities { return ports.Capabilities{ports.CapCob: true} }

func (soCob) CriarCobv(context.Context, domain.Conta, domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	return domain.CobrancaVencimento{}, ports.ErrNaoSuportado
}

func (soCob) Devolver(context.Context, domain.Conta, domain.Devolucao) (domain.Devolucao, error) {
	return domain.Devolucao{}, ports.ErrNaoSuportado
}

func (soCob) ConfigurarWebhook(context.Context, domain.Conta, string) error {
	return ports.ErrNaoSuportado
}

func (soCob) RemoverWebhook(context.Context, domain.Conta) error {
	return ports.ErrNaoSuportado
}

// A suíte pula só o que o adapter não declara: um adapter que só faz cob passa
// inteiro, com os cenários de cob rodando.
func TestAdapterSoComCobPassa(t *testing.T) {
	conformance.Run(t, func(*testing.T) conformance.Alvo {
		clock := fake.NewClock(time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC))
		return conformance.Alvo{
			PSP:   soCob{sandbox.New(clock)},
			Conta: domain.Conta{ID: "loja", PSP: "so-cob", ChavePix: "x@example.com", Ambiente: domain.Homologacao, Ativa: true},
			Agora: clock.Now,
			Pagar: func(*testing.T, domain.Txid) { clock.Avancar(sandbox.AtrasoPadrao) },
		}
	})
}
