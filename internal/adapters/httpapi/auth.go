package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
)

// escopoDaRota diz o escopo exigido por cada rota do contrato (RN-60).
// Rota fora deste mapa é recusada: esquecer de mapear uma rota nova falha
// fechado, e o teste TestTodaRotaTemEscopo aponta qual faltou.
var escopoDaRota = map[string]domain.Escopo{
	"GET /cob/{txid}":                       domain.EscopoCobRead,
	"GET /cob":                              domain.EscopoCobRead,
	"PUT /cob/{txid}":                       domain.EscopoCobWrite,
	"POST /cob":                             domain.EscopoCobWrite,
	"PATCH /cob/{txid}":                     domain.EscopoCobWrite,
	"GET /cobv/{txid}":                      domain.EscopoCobvRead,
	"GET /cobv":                             domain.EscopoCobvRead,
	"PUT /cobv/{txid}":                      domain.EscopoCobvWrite,
	"PATCH /cobv/{txid}":                    domain.EscopoCobvWrite,
	"GET /vintem/v1/contas":                 domain.EscopoContaAdmin,
	"POST /vintem/v1/contas":                domain.EscopoContaAdmin,
	"GET /vintem/v1/contas/{id}":            domain.EscopoContaAdmin,
	"POST /vintem/v1/contas/{id}/desativar": domain.EscopoContaAdmin,
	"POST /rec":                             domain.EscopoRecWrite,
	"GET /rec/{idRec}":                      domain.EscopoRecRead,
	"POST /solicrec":                        domain.EscopoRecWrite,
	"GET /solicrec/{idSolicRec}":            domain.EscopoRecRead,
	"PATCH /solicrec/{idSolicRec}":          domain.EscopoRecWrite,
	"POST /cobr":                            domain.EscopoCobrWrite,
	"PUT /cobr/{txid}":                      domain.EscopoCobrWrite,
	"GET /cobr/{txid}":                      domain.EscopoCobrRead,
	"PATCH /cobr/{txid}":                    domain.EscopoCobrWrite,
	"POST /cobr/{txid}/retentativa/{data}":  domain.EscopoCobrWrite,
	"GET /pix":                              domain.EscopoPixRead,
	"GET /pix/{e2eid}":                      domain.EscopoPixRead,
	"PUT /pix/{e2eid}/devolucao/{id}":       domain.EscopoDevolucaoWrite,
	"GET /pix/{e2eid}/devolucao/{id}":       domain.EscopoPixRead,
	"GET /vintem/v1/webhooks/entregas":      domain.EscopoWebhookAdmin,
	"POST /vintem/v1/webhooks/entregas/{eventoId}/reenviar": domain.EscopoWebhookAdmin,
}

// rotasPublicas não exigem API key. Os webhooks são chamados pelo banco e
// autenticados pelo token da URL, conferido no caso de uso.
var rotasPublicas = map[string]bool{
	"GET /health":                                true,
	"POST /webhooks/pix/{contaId}/{token}":       true,
	"POST /webhooks/pix/{contaId}/{token}/pix":   true,
	"POST /webhooks/rec/{contaId}/{token}":       true,
	"POST /webhooks/rec/{contaId}/{token}/rec":   true,
	"POST /webhooks/cobr/{contaId}/{token}":      true,
	"POST /webhooks/cobr/{contaId}/{token}/cobr": true,
}

// autenticar exige "Authorization: Bearer vtm_..." com o escopo da rota, e
// anexa o acesso (conta restrita, se houver) ao contexto dos casos de uso.
func autenticar(keys app.APIKeys) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rotasPublicas[r.Pattern] {
				next.ServeHTTP(w, r)
				return
			}
			escopo, ok := escopoDaRota[r.Pattern]
			if !ok {
				escreverProblema(w, problemaDe(r.Context(), errors.New("rota sem escopo mapeado: "+r.Pattern)))
				return
			}
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				escreverProblema(w, problemaDe(r.Context(), domain.ErrNaoAutenticado))
				return
			}
			k, err := keys.Autenticar(r.Context(), strings.TrimSpace(token))
			if err != nil {
				escreverProblema(w, problemaDe(r.Context(), err))
				return
			}
			if err := k.Permite(escopo); err != nil {
				escreverProblema(w, problemaDe(r.Context(), err))
				return
			}
			next.ServeHTTP(w, r.WithContext(app.ComAcesso(r.Context(), app.AcessoDaChave(k))))
		})
	}
}
