// Package efitest é um Efí de mentira para testes: TLS que exige o certificado
// do cliente, token OAuth2 por Basic auth e corpo JSON, e cob em memória. As
// respostas seguem os exemplos da documentação do Efí, inclusive os desvios
// (erro {"nome","mensagem"} e expiracao como string no GET). Não serve para produção.
package efitest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vintemhq/vintem/pkg/brcode"
)

// Servidor é o Efí falso.
type Servidor struct {
	URL string
	// Raizes confia no certificado TLS do servidor (passe em efi.ComRaizes).
	Raizes *x509.CertPool

	clientID, clientSecret string

	mu             sync.Mutex
	tokens         map[string]bool
	seq            int
	cobs           map[string]map[string]any
	cobvs          map[string]map[string]any
	pix            []map[string]any
	devolucoes     map[string]map[string]any // chave: e2e/id
	webhooks       map[string]string         // chave Pix → webhookUrl
	tokensEmitidos int
	chamadas       []string
	falhas         []int // status forçados para as próximas chamadas à API (não ao token)
}

// Novo sobe o servidor. Só aceita conexões com um certificado de cliente
// emitido por clienteCA, e tokens pedidos com clientID e clientSecret.
func Novo(t testing.TB, clienteCA *x509.Certificate, clientID, clientSecret string) *Servidor {
	t.Helper()
	s := &Servidor{clientID: clientID, clientSecret: clientSecret, tokens: map[string]bool{}, cobs: map[string]map[string]any{}, cobvs: map[string]map[string]any{}, devolucoes: map[string]map[string]any{}, webhooks: map[string]string{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(s.servir))
	pool := x509.NewCertPool()
	pool.AddCert(clienteCA)
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	s.URL = srv.URL
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

// Chamadas lista "MÉTODO caminho" de cada chamada à API, em ordem.
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

// Falhar faz as próximas chamadas à API responderem os status dados, em ordem.
func (s *Servidor) Falhar(status ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.falhas = append(s.falhas, status...)
}

// Pagar conclui a cob ou cobv do txid e registra o Pix recebido no horário
// dado, como a homologação do Efí faz com valores de até R$ 10,00. Devolve o
// endToEndId, ou "" se a cobrança não estiver ATIVA (removida não é paga).
func (s *Servidor) Pagar(txid string, horario time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cobs[txid]
	if c == nil {
		c = s.cobvs[txid]
	}
	if c == nil {
		panic("efitest: Pagar de txid inexistente " + txid)
	}
	if c["status"] != "ATIVA" {
		return ""
	}
	c["status"] = "CONCLUIDA"
	valor := c["valor"].(map[string]any)["original"].(string)
	return s.registrarPix(txid, valor, fmt.Sprint(c["chave"]), horario)
}

// AdicionarPix registra n Pix por chave, sem cobrança, a partir de horario, um
// por segundo. Serve para testar a paginação de GET /v2/pix.
func (s *Servidor) AdicionarPix(n int, horario time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range n {
		s.registrarPix("", "1.00", "loja@example.com", horario.Add(time.Duration(i)*time.Second))
	}
}

func (s *Servidor) registrarPix(txid, valor, chave string, horario time.Time) string {
	s.seq++
	e2e := fmt.Sprintf("E18236120%015d%08d", horario.Unix(), s.seq)[:32]
	p := map[string]any{
		"endToEndId": e2e, "valor": valor, "chave": chave, "horario": horario.UTC().Format(time.RFC3339Nano),
		"infoPagador": "pagando o pix", "pagador": map[string]any{"cpf": "12345678909", "nome": "Fulano Pagador"},
	}
	if txid != "" {
		p["txid"] = txid
	}
	s.pix = append(s.pix, p)
	return e2e
}

// problema responde no formato de erro da spec do BCB, que o Efí usa na cobv.
func problema(w http.ResponseWriter, status int, titulo, detalhe string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "https://pix.bcb.gov.br/api/v2/error/CobVOperacaoInvalida", "title": titulo, "status": status, "detail": detalhe})
}

func erro(w http.ResponseWriter, status int, nome, mensagem string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"nome": nome, "mensagem": mensagem})
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

func (s *Servidor) servir(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/oauth/token" {
		s.token(w, r)
		return
	}
	s.mu.Lock()
	s.chamadas = append(s.chamadas, r.Method+" "+r.URL.Path)
	var forcado int
	if len(s.falhas) > 0 {
		forcado, s.falhas = s.falhas[0], s.falhas[1:]
	}
	autorizado := s.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	s.mu.Unlock()

	switch {
	case !autorizado:
		erro(w, http.StatusUnauthorized, "nao_autorizado", "token ausente, expirado ou revogado")
		return
	case forcado != 0:
		erro(w, forcado, "erro_aplicacao", "falha simulada")
		return
	}
	if txid, ok := strings.CutPrefix(r.URL.Path, "/v2/cob/"); ok {
		s.cob(w, r, txid)
		return
	}
	if txid, ok := strings.CutPrefix(r.URL.Path, "/v2/cobv/"); ok {
		s.cobv(w, r, txid)
		return
	}
	if chave, ok := strings.CutPrefix(r.URL.Path, "/v2/webhook/"); ok {
		s.webhook(w, r, chave)
		return
	}
	if r.URL.Path == "/v2/pix" {
		s.listarPix(w, r)
		return
	}
	if resto, ok := strings.CutPrefix(r.URL.Path, "/v2/pix/"); ok {
		if e2e, id, ok := strings.Cut(resto, "/devolucao/"); ok {
			s.devolucao(w, r, e2e, id)
			return
		}
	}
	erro(w, http.StatusNotFound, "rota_nao_encontrada", "rota não existe no Efí falso")
}

func (s *Servidor) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	var corpo struct {
		GrantType string `json:"grant_type"`
	}
	switch {
	case r.Method != http.MethodPost:
		erro(w, http.StatusMethodNotAllowed, "metodo", "use POST")
	case !ok || id != s.clientID || secret != s.clientSecret:
		erro(w, http.StatusUnauthorized, "invalid_client", "credenciais inválidas")
	case r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&corpo) != nil || corpo.GrantType != "client_credentials":
		erro(w, http.StatusBadRequest, "invalid_request", `corpo deve ser JSON {"grant_type":"client_credentials"}`)
	default:
		s.mu.Lock()
		s.seq++
		s.tokensEmitidos++
		token := fmt.Sprintf("token-efi-%d", s.seq)
		s.tokens[token] = true
		s.mu.Unlock()
		responder(w, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600, "scope": "cob.write cob.read"})
	}
}

func (s *Servidor) cob(w http.ResponseWriter, r *http.Request, txid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	atual := s.cobs[txid]
	switch r.Method {
	case http.MethodPut:
		if atual != nil {
			erro(w, http.StatusConflict, "txid_duplicado", "Campo txid informado já foi utilizado em outra cobrança")
			return
		}
		var corpo map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &corpo) != nil {
			erro(w, http.StatusBadRequest, "json_invalido", "corpo não é JSON")
			return
		}
		s.seq++
		location := fmt.Sprintf("pix-h.example.efipay.com.br/v2/%d", s.seq)
		corpo["txid"], corpo["status"], corpo["revisao"], corpo["location"] = txid, "ATIVA", 0, location
		corpo["loc"] = map[string]any{"id": s.seq, "location": location, "tipoCob": "cob"}
		cal, _ := corpo["calendario"].(map[string]any)
		if cal == nil {
			cal = map[string]any{"expiracao": 3600}
		}
		cal["criacao"] = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		corpo["calendario"] = cal
		corpo["pixCopiaECola"] = brCodeDinamico(location)
		s.cobs[txid] = corpo
		responder(w, http.StatusCreated, corpo)
	case http.MethodGet:
		if atual == nil {
			erro(w, http.StatusNotFound, "cobranca_nao_encontrada", "Nenhuma cobrança encontrada para o txid informado")
			return
		}
		// Desvio documentado: no GET, expiracao vem como string.
		copia := map[string]any{}
		for k, v := range atual {
			copia[k] = v
		}
		if cal, ok := atual["calendario"].(map[string]any); ok {
			c := map[string]any{"criacao": cal["criacao"], "expiracao": fmt.Sprint(cal["expiracao"])}
			copia["calendario"] = c
		}
		responder(w, http.StatusOK, copia)
	case http.MethodPatch:
		if atual == nil {
			erro(w, http.StatusNotFound, "cobranca_nao_encontrada", "Nenhuma cobrança encontrada para o txid informado")
			return
		}
		if atual["status"] != "ATIVA" {
			erro(w, http.StatusBadRequest, "status_cobranca_invalido", "A cobrança não está mais com o status ATIVA")
			return
		}
		var mudancas map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &mudancas) != nil {
			erro(w, http.StatusBadRequest, "json_invalido", "corpo não é JSON")
			return
		}
		for k, v := range mudancas {
			if k == "calendario" {
				cal, _ := atual["calendario"].(map[string]any)
				for ck, cv := range v.(map[string]any) {
					cal[ck] = cv
				}
				continue
			}
			atual[k] = v
		}
		atual["revisao"] = atual["revisao"].(int) + 1
		responder(w, http.StatusOK, atual)
	default:
		erro(w, http.StatusMethodNotAllowed, "metodo", "método não suportado")
	}
}

// brCodeDinamico gera o "Pix Copia e Cola" que o Efí devolveria para a location.
func brCodeDinamico(location string) string {
	code, err := brcode.Encode(brcode.Pix{URL: location, Nome: "EFI TESTE", Cidade: "SAO PAULO"})
	if err != nil {
		panic(err) // location gerada aqui mesmo: só falha se o código estiver errado
	}
	return code
}

func (s *Servidor) cobv(w http.ResponseWriter, r *http.Request, txid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	atual := s.cobvs[txid]
	switch r.Method {
	case http.MethodPut:
		var corpo map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &corpo) != nil {
			problema(w, http.StatusBadRequest, "Cobrança inválida.", "corpo não é JSON")
			return
		}
		if atual != nil {
			// Como na homologação (29/10/2026): PUT num txid existente não dá
			// erro, sobrescreve a cobv e soma uma revisão.
			for k, v := range corpo {
				atual[k] = v
			}
			atual["revisao"] = atual["revisao"].(int) + 1
			responder(w, http.StatusCreated, atual)
			return
		}
		s.seq++
		location := fmt.Sprintf("pix-h.example.efipay.com.br/v2/cobv/%d", s.seq)
		// Como o Efí de verdade: a cobv só traz a location dentro de loc.
		corpo["txid"], corpo["status"], corpo["revisao"] = txid, "ATIVA", 0
		corpo["loc"] = map[string]any{"id": s.seq, "location": location, "tipoCob": "cobv"}
		corpo["recebedor"] = map[string]any{"cnpj": "12682781000116", "nome": "Loja Teste"}
		cal, _ := corpo["calendario"].(map[string]any)
		cal["criacao"] = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		corpo["pixCopiaECola"] = brCodeDinamico(location)
		s.cobvs[txid] = corpo
		responder(w, http.StatusCreated, corpo)
	case http.MethodGet:
		if atual == nil {
			problema(w, http.StatusNotFound, "Não Encontrado", "Nenhuma cobrança encontrada para o txid informado.")
			return
		}
		responder(w, http.StatusOK, atual)
	case http.MethodPatch:
		if atual == nil {
			problema(w, http.StatusNotFound, "Não Encontrado", "Nenhuma cobrança encontrada para o txid informado.")
			return
		}
		if atual["status"] != "ATIVA" {
			problema(w, http.StatusBadRequest, "Operação inválida.", "Somente cobranças ATIVA podem ser revisadas.")
			return
		}
		var mudancas map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &mudancas) != nil {
			problema(w, http.StatusBadRequest, "Operação inválida.", "corpo não é JSON")
			return
		}
		for k, v := range mudancas {
			atual[k] = v
		}
		atual["revisao"] = atual["revisao"].(int) + 1
		responder(w, http.StatusOK, atual)
	default:
		problema(w, http.StatusMethodNotAllowed, "Método", "método não suportado")
	}
}

func (s *Servidor) listarPix(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	inicio, err1 := time.Parse(time.RFC3339, q.Get("inicio"))
	fim, err2 := time.Parse(time.RFC3339, q.Get("fim"))
	if err1 != nil || err2 != nil {
		erro(w, http.StatusBadRequest, "parametro_invalido", "inicio e fim são obrigatórios, em RFC 3339")
		return
	}
	pagina, _ := strconv.Atoi(q.Get("paginacao.paginaAtual"))
	porPagina, err := strconv.Atoi(q.Get("paginacao.itensPorPagina"))
	if err != nil || porPagina <= 0 {
		porPagina = 100
	}
	s.mu.Lock()
	var filtrados []map[string]any
	for _, p := range s.pix {
		h, _ := time.Parse(time.RFC3339, p["horario"].(string))
		if !h.Before(inicio) && !h.After(fim) { // o Efí filtra com fim inclusivo
			filtrados = append(filtrados, p)
		}
	}
	s.mu.Unlock()
	paginas := (len(filtrados) + porPagina - 1) / porPagina
	de, ate := min(pagina*porPagina, len(filtrados)), min((pagina+1)*porPagina, len(filtrados))
	responder(w, http.StatusOK, map[string]any{
		"parametros": map[string]any{"inicio": q.Get("inicio"), "fim": q.Get("fim"), "paginacao": map[string]any{
			"paginaAtual": pagina, "itensPorPagina": porPagina, "quantidadeDePaginas": paginas, "quantidadeTotalDeItens": len(filtrados),
		}},
		"pix": append([]map[string]any{}, filtrados[de:ate]...),
	})
}

func (s *Servidor) devolucao(w http.ResponseWriter, r *http.Request, e2e, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	chave := e2e + "/" + id
	if r.Method == http.MethodGet {
		d := s.devolucoes[chave]
		if d == nil {
			// Como na homologação (29/10/2026): 400, não 404.
			erro(w, http.StatusBadRequest, "devolucao_nao_encontrada", "Nenhuma devolução encontrada para o identificador informado")
			return
		}
		responder(w, http.StatusOK, d)
		return
	}
	if r.Method != http.MethodPut {
		erro(w, http.StatusMethodNotAllowed, "metodo", "método não suportado")
		return
	}
	var pix map[string]any
	for _, p := range s.pix {
		if p["endToEndId"] == e2e {
			pix = p
		}
	}
	if pix == nil {
		erro(w, http.StatusBadRequest, "pix_nao_encontrado", "Nenhum pix encontrado para o identificador informado")
		return
	}
	if s.devolucoes[chave] != nil {
		erro(w, http.StatusConflict, "devolucao_id_duplicado", "O id informado já foi utilizado em outra devolução")
		return
	}
	var corpo struct {
		Valor     string `json:"valor"`
		Descricao string `json:"descricao"`
	}
	if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &corpo) != nil {
		erro(w, http.StatusBadRequest, "json_invalido", "corpo não é JSON")
		return
	}
	// Como em produção (30/10/2026): o campo descricao é recusado.
	if corpo.Descricao != "" {
		erro(w, http.StatusBadRequest, "json_invalido", "Valores ou tipos de campo inválidos")
		return
	}
	pedido, _ := strconv.ParseFloat(corpo.Valor, 64)
	recebido, _ := strconv.ParseFloat(pix["valor"].(string), 64)
	devolvido := 0.0
	for k, d := range s.devolucoes {
		if strings.HasPrefix(k, e2e+"/") {
			v, _ := strconv.ParseFloat(d["valor"].(string), 64)
			devolvido += v
		}
	}
	if pedido <= 0 || devolvido+pedido > recebido+1e-9 {
		// Nome de erro não documentado pelo Efí: só importa que a devolução seja recusada.
		erro(w, http.StatusBadRequest, "valor_devolucao_invalido", "O valor da devolução passa do valor disponível do Pix")
		return
	}
	s.seq++
	d := map[string]any{
		"id": id, "rtrId": fmt.Sprintf("D18236120202010291200%011d", s.seq), "valor": corpo.Valor, "status": "EM_PROCESSAMENTO",
		"horario": map[string]any{"solicitacao": time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC).Format(time.RFC3339)},
	}
	s.devolucoes[chave] = d
	responder(w, http.StatusCreated, d)
}

// LiquidarDevolucao leva a devolução a DEVOLVIDO, com horário de liquidação,
// como o banco faz depois de responder EM_PROCESSAMENTO.
func (s *Servidor) LiquidarDevolucao(e2e, id string, horario time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.devolucoes[e2e+"/"+id]
	d["status"] = "DEVOLVIDO"
	d["horario"].(map[string]any)["liquidacao"] = horario.Format(time.RFC3339)
}

// Webhook devolve a URL cadastrada para a chave Pix, ou "" se não houver.
func (s *Servidor) Webhook(chave string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.webhooks[chave]
}

func (s *Servidor) webhook(w http.ResponseWriter, r *http.Request, chave string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodPut:
		// O Efí exige mTLS no servidor do webhook; sem o skip, o servidor do Vintém
		// (que não termina TLS) seria recusado. O nome do erro é deste falso.
		if r.Header.Get("x-skip-mtls-checking") != "true" {
			erro(w, http.StatusBadRequest, "webhook_invalido", "A URL do webhook precisa aceitar mTLS com o certificado do Efí")
			return
		}
		var corpo struct {
			WebhookURL string `json:"webhookUrl"`
		}
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &corpo) != nil || !strings.HasPrefix(corpo.WebhookURL, "https://") {
			erro(w, http.StatusBadRequest, "webhook_invalido", "webhookUrl deve ser https")
			return
		}
		s.webhooks[chave] = corpo.WebhookURL
		responder(w, http.StatusOK, map[string]string{"webhookUrl": corpo.WebhookURL})
	case http.MethodGet:
		u, ok := s.webhooks[chave]
		if !ok {
			erro(w, http.StatusNotFound, "webhook_nao_encontrado", "Nenhum webhook encontrado para a chave informada")
			return
		}
		responder(w, http.StatusOK, map[string]string{"webhookUrl": u, "chave": chave})
	case http.MethodDelete:
		if _, ok := s.webhooks[chave]; !ok {
			erro(w, http.StatusNotFound, "webhook_nao_encontrado", "Nenhum webhook encontrado para a chave informada")
			return
		}
		delete(s.webhooks, chave)
		w.WriteHeader(http.StatusNoContent)
	default:
		erro(w, http.StatusMethodNotAllowed, "metodo", "método não suportado")
	}
}
