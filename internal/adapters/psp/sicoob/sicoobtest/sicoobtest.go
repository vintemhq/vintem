// Package sicoobtest é um Sicoob de mentira para testes, escrito pela spec do
// BCB com os desvios que o adapter trata: TLS que exige o certificado do
// cliente, token OAuth2 por formulário só com client_id e scope (sem client
// secret), header client_id em toda chamada, rotas sob /pix/api/v2 e o BR
// Code no campo brcode. Não serve para produção.
package sicoobtest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/pkg/brcode"
)

// Servidor é o Sicoob falso.
type Servidor struct {
	// URL é a base da API Pix (termina em /pix/api/v2), como as URLs do adapter.
	URL string
	// Raizes confia no certificado TLS do servidor (passe em sicoob.ComRaizes).
	Raizes *x509.CertPool

	clientID string

	mu             sync.Mutex
	tokens         map[string]bool
	seq            int
	cobs, cobvs    map[string]map[string]any
	pix            []map[string]any
	devolucoes     map[string]map[string]any // chave: e2e/id
	webhooks       map[string]string
	tokensEmitidos int
	chamadas       []string
	escopos        string
}

// Novo sobe o servidor. Só aceita conexões com um certificado de cliente
// emitido por clienteCA, e tokens e chamadas com clientID.
func Novo(t testing.TB, clienteCA *x509.Certificate, clientID string) *Servidor {
	t.Helper()
	s := &Servidor{clientID: clientID, tokens: map[string]bool{},
		cobs: map[string]map[string]any{}, cobvs: map[string]map[string]any{}, devolucoes: map[string]map[string]any{}, webhooks: map[string]string{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(s.servir))
	pool := x509.NewCertPool()
	pool.AddCert(clienteCA)
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	s.URL = srv.URL + "/pix/api/v2"
	s.Raizes = x509.NewCertPool()
	s.Raizes.AddCert(srv.Certificate())
	return s
}

// TokensEmitidos conta quantos tokens o servidor entregou.
func (s *Servidor) TokensEmitidos() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokensEmitidos
}

// EscoposPedidos é o scope do último pedido de token.
func (s *Servidor) EscoposPedidos() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.escopos
}

// Chamadas lista "MÉTODO caminho" de cada chamada à API Pix, em ordem.
func (s *Servidor) Chamadas() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.chamadas...)
}

// RevogarTokens invalida os tokens emitidos: a próxima chamada recebe 401.
func (s *Servidor) RevogarTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = map[string]bool{}
}

// Webhook devolve a URL registrada na chave, ou "".
func (s *Servidor) Webhook(chave string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.webhooks[chave]
}

// Pagar conclui a cob ou cobv ATIVA do txid e registra o Pix no horário dado.
// Devolve o endToEndId, ou "" se a cobrança não estiver ATIVA.
func (s *Servidor) Pagar(txid string, horario time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pagar(txid, horario)
}

func (s *Servidor) pagar(txid string, horario time.Time) string {
	c := s.cobs[txid]
	if c == nil {
		c = s.cobvs[txid]
	}
	if c == nil || c["status"] != "ATIVA" {
		return ""
	}
	c["status"] = "CONCLUIDA"
	s.seq++
	e2e := fmt.Sprintf("E02038232%015d%08d", horario.Unix(), s.seq)[:32]
	s.pix = append(s.pix, map[string]any{
		"endToEndId": e2e, "txid": txid, "valor": c["valor"].(map[string]any)["original"], "chave": c["chave"],
		"horario": horario.UTC().Format(time.RFC3339Nano), "infoPagador": "pagamento no sandbox",
	})
	return e2e
}

// problema responde no formato de erro da spec do BCB, que o Sicoob segue.
func problema(w http.ResponseWriter, status int, tipo, detalhe string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "https://pix.bcb.gov.br/api/v2/error/" + tipo, "title": tipo, "status": status, "detail": detalhe})
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

func (s *Servidor) servir(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/auth/realms/cooperado/protocol/openid-connect/token" {
		s.token(w, r)
		return
	}
	caminho, ok := strings.CutPrefix(r.URL.Path, "/pix/api/v2")
	if !ok {
		problema(w, http.StatusNotFound, "NaoEncontrado", "rota não existe no Sicoob falso")
		return
	}
	if r.Header.Get("client_id") != s.clientID {
		// O gateway do Sicoob recusa antes da API, no formato dele.
		responder(w, http.StatusUnauthorized, map[string]any{"httpCode": "401", "httpMessage": "Unauthorized", "moreInformation": "header client_id ausente ou inválido"})
		return
	}
	s.mu.Lock()
	s.chamadas = append(s.chamadas, r.Method+" "+caminho)
	autorizado := s.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	s.mu.Unlock()
	if !autorizado {
		problema(w, http.StatusUnauthorized, "AcessoNegado", "token ausente, expirado ou revogado")
		return
	}
	var corpo map[string]any
	if r.Body != nil {
		if b, _ := io.ReadAll(r.Body); len(b) > 0 && json.Unmarshal(b, &corpo) != nil {
			problema(w, http.StatusBadRequest, "RequisicaoInvalida", "corpo não é JSON")
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case strings.HasPrefix(caminho, "/cob/"):
		s.cobranca(w, r, s.cobs, "cob", strings.TrimPrefix(caminho, "/cob/"), corpo)
	case strings.HasPrefix(caminho, "/cobv/"):
		s.cobranca(w, r, s.cobvs, "cobv", strings.TrimPrefix(caminho, "/cobv/"), corpo)
	case caminho == "/pix" && r.Method == http.MethodGet:
		s.listarPix(w, r)
	case strings.HasPrefix(caminho, "/pix/") && strings.Contains(caminho, "/devolucao/"):
		e2e, id, _ := strings.Cut(strings.TrimPrefix(caminho, "/pix/"), "/devolucao/")
		s.devolucao(w, r, e2e, id, corpo)
	case strings.HasPrefix(caminho, "/webhook/") && r.Method == http.MethodPut:
		url, _ := corpo["webhookUrl"].(string)
		if !strings.HasPrefix(url, "https://") {
			problema(w, http.StatusBadRequest, "WebhookOperacaoInvalida", "webhookUrl precisa ser https")
			return
		}
		s.webhooks[strings.TrimPrefix(caminho, "/webhook/")] = url
		w.WriteHeader(http.StatusNoContent)
	case strings.HasPrefix(caminho, "/webhook/") && r.Method == http.MethodDelete:
		chave := strings.TrimPrefix(caminho, "/webhook/")
		if _, ok := s.webhooks[chave]; !ok {
			problema(w, http.StatusNotFound, "NaoEncontrado", "nenhum webhook na chave")
			return
		}
		delete(s.webhooks, chave)
		w.WriteHeader(http.StatusNoContent)
	default:
		problema(w, http.StatusNotFound, "NaoEncontrado", "rota não existe no Sicoob falso")
	}
}

func (s *Servidor) token(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || r.ParseForm() != nil {
		responder(w, http.StatusBadRequest, map[string]any{"error": "invalid_request", "error_description": "use POST com corpo application/x-www-form-urlencoded"})
		return
	}
	f := r.PostForm
	if f.Get("client_id") != s.clientID || f.Get("grant_type") != "client_credentials" {
		responder(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client", "error_description": "Invalid client credentials"})
		return
	}
	s.mu.Lock()
	s.seq++
	s.tokensEmitidos++
	s.escopos = f.Get("scope")
	token := fmt.Sprintf("token-sicoob-%d", s.seq)
	s.tokens[token] = true
	s.mu.Unlock()
	responder(w, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 300, "scope": f.Get("scope")})
}

// cobranca atende cob e cobv: PUT cria (txid repetido é 400; o Sicoob não
// documenta o erro), GET consulta, PATCH revisa ou remove. O BR Code volta em
// brcode, não em pixCopiaECola.
func (s *Servidor) cobranca(w http.ResponseWriter, r *http.Request, mapa map[string]map[string]any, tipo, txid string, corpo map[string]any) {
	atual := mapa[txid]
	invalida := map[string]string{"cob": "CobOperacaoInvalida", "cobv": "CobVOperacaoInvalida"}[tipo]
	switch r.Method {
	case http.MethodPut:
		if atual != nil {
			problema(w, http.StatusBadRequest, invalida, "txid já utilizado")
			return
		}
		s.seq++
		if tipo == "cob" {
			if d, _ := corpo["devedor"].(map[string]any); d != nil {
				for k := range d {
					if k != "cpf" && k != "cnpj" && k != "nome" {
						problema(w, http.StatusBadRequest, "CobOperacaoInvalida", "devedor da cob só aceita cpf, cnpj e nome")
						return
					}
				}
			}
		}
		location := fmt.Sprintf("pix.sicoob.com.br/qr/payload/v2/%s/%d", tipo, s.seq)
		corpo["txid"], corpo["status"], corpo["revisao"] = txid, "ATIVA", 0
		corpo["location"] = location
		corpo["loc"] = map[string]any{"id": s.seq, "location": location, "tipoCob": tipo}
		cal, _ := corpo["calendario"].(map[string]any)
		if cal == nil {
			cal = map[string]any{}
			corpo["calendario"] = cal
		}
		cal["criacao"] = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		code, err := brcode.Encode(brcode.Pix{URL: location, Nome: "SICOOB TESTE", Cidade: "BRASILIA"})
		if err != nil {
			panic(err) // location gerada aqui: só falha se o código estiver errado
		}
		corpo["brcode"] = code
		mapa[txid] = corpo
		responder(w, http.StatusCreated, corpo)
	case http.MethodGet:
		if atual == nil {
			problema(w, http.StatusNotFound, "NaoEncontrado", "Entidade não encontrada.")
			return
		}
		responder(w, http.StatusOK, atual)
	case http.MethodPatch:
		if atual == nil {
			problema(w, http.StatusNotFound, "NaoEncontrado", "Entidade não encontrada.")
			return
		}
		if atual["status"] != "ATIVA" {
			problema(w, http.StatusBadRequest, invalida, "a cobrança não está ATIVA")
			return
		}
		for k, v := range corpo {
			if k == "valor" {
				valor := atual["valor"].(map[string]any)
				for vk, vv := range v.(map[string]any) {
					valor[vk] = vv
				}
				continue
			}
			atual[k] = v
		}
		atual["revisao"] = atual["revisao"].(int) + 1
		responder(w, http.StatusOK, atual)
	default:
		problema(w, http.StatusMethodNotAllowed, "RequisicaoInvalida", "método não suportado")
	}
}

func (s *Servidor) listarPix(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	inicio, err1 := time.Parse(time.RFC3339, q.Get("inicio"))
	fim, err2 := time.Parse(time.RFC3339, q.Get("fim"))
	if err1 != nil || err2 != nil {
		problema(w, http.StatusBadRequest, "ConsultaInvalida", "inicio e fim são obrigatórios, em RFC 3339")
		return
	}
	pagina, _ := strconv.Atoi(q.Get("paginacao.paginaAtual"))
	porPagina, _ := strconv.Atoi(q.Get("paginacao.itensPorPagina"))
	if porPagina <= 0 {
		porPagina = 100
	}
	var na []map[string]any
	for _, p := range s.pix {
		h, _ := time.Parse(time.RFC3339Nano, p["horario"].(string))
		if !h.Before(inicio) && !h.After(fim) {
			na = append(na, p)
		}
	}
	sort.SliceStable(na, func(i, j int) bool { return na[i]["horario"].(string) < na[j]["horario"].(string) })
	paginas := max(1, (len(na)+porPagina-1)/porPagina)
	ini, fimPag := min(pagina*porPagina, len(na)), min((pagina+1)*porPagina, len(na))
	responder(w, http.StatusOK, map[string]any{
		"parametros": map[string]any{"inicio": q.Get("inicio"), "fim": q.Get("fim"),
			"paginacao": map[string]any{"paginaAtual": pagina, "itensPorPagina": porPagina, "quantidadeDePaginas": paginas, "quantidadeTotalDeItens": len(na)}},
		"pix": na[ini:fimPag],
	})
}

func (s *Servidor) devolucao(w http.ResponseWriter, r *http.Request, e2e, id string, corpo map[string]any) {
	chave := e2e + "/" + id
	switch r.Method {
	case http.MethodGet:
		d := s.devolucoes[chave]
		if d == nil {
			problema(w, http.StatusNotFound, "NaoEncontrado", "Entidade não encontrada.")
			return
		}
		responder(w, http.StatusOK, d)
	case http.MethodPut:
		var recebido map[string]any
		for _, p := range s.pix {
			if p["endToEndId"] == e2e {
				recebido = p
			}
		}
		if recebido == nil {
			problema(w, http.StatusNotFound, "NaoEncontrado", "Pix não encontrado.")
			return
		}
		if s.devolucoes[chave] != nil {
			problema(w, http.StatusBadRequest, "PixDevolucaoInvalida", "id de devolução já utilizado")
			return
		}
		pedido, err := domain.ParseMoney(fmt.Sprint(corpo["valor"]))
		total, _ := domain.ParseMoney(fmt.Sprint(recebido["valor"]))
		var devolvido domain.Money
		for k, d := range s.devolucoes {
			if strings.HasPrefix(k, e2e+"/") {
				v, _ := domain.ParseMoney(d["valor"].(string))
				devolvido += v
			}
		}
		if err != nil || pedido <= 0 || devolvido+pedido > total {
			problema(w, http.StatusBadRequest, "PixDevolucaoInvalida", "valor maior que o disponível para devolução")
			return
		}
		d := map[string]any{"id": id, "rtrId": fmt.Sprintf("D02038232%023d", s.seq), "valor": pedido.String(),
			"horario": map[string]any{"solicitacao": time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)}, "status": "EM_PROCESSAMENTO"}
		s.devolucoes[chave] = d
		responder(w, http.StatusCreated, d)
	default:
		problema(w, http.StatusMethodNotAllowed, "RequisicaoInvalida", "método não suportado")
	}
}
