// Package e2e roda o binário do vintem em modo sandbox, de ponta a ponta
// (docs/ARQUITETURA.md, "Testes").
package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"
)

// binario é o vintem compilado uma vez para a suíte, como o usuário o roda.
var binario string

func TestMain(m *testing.M) {
	os.Exit(rodar(m))
}

func rodar(m *testing.M) int {
	dir, err := os.MkdirTemp("", "vintem-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = os.RemoveAll(dir) }()
	binario = filepath.Join(dir, "vintem")
	build := exec.CommandContext(context.Background(), "go", "build", "-o", binario, "github.com/vintemhq/vintem/cmd/vintem") //nolint:gosec // G204: comando fixo da suíte
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "compilando o vintem:", err)
		return 1
	}
	return m.Run()
}

// evento é o que o destino recebeu, já com a assinatura conferida.
type evento struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// destino é a aplicação do cliente: só aceita evento com assinatura válida
// (Standard Webhooks, biblioteca oficial) e guarda o que chegou.
type destino struct {
	srv       *httptest.Server
	mu        sync.Mutex
	whsec     string
	eventos   []evento
	invalidos int
	repetidos int
	idsVistos map[string]bool
}

func novoDestino(t *testing.T) *destino {
	d := &destino{idsVistos: map[string]bool{}}
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		d.mu.Lock()
		defer d.mu.Unlock()
		wh, err := standardwebhooks.NewWebhook(d.whsec)
		if err != nil || wh.Verify(corpo, r.Header) != nil {
			d.invalidos++
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if id := r.Header.Get("webhook-id"); d.idsVistos[id] {
			d.repetidos++
		} else {
			d.idsVistos[id] = true
		}
		var e evento
		if err := json.Unmarshal(corpo, &e); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		d.eventos = append(d.eventos, e)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(d.srv.Close)
	return d
}

// esperar espera chegar um evento do tipo cujo data tem o campo com o valor.
func (d *destino) esperar(t *testing.T, tipo, campo, valor string) map[string]any {
	t.Helper()
	limite := time.Now().Add(30 * time.Second)
	for time.Now().Before(limite) {
		d.mu.Lock()
		for _, e := range d.eventos {
			var dados map[string]any
			if e.Type == tipo && json.Unmarshal(e.Data, &dados) == nil && dados[campo] == valor {
				d.mu.Unlock()
				return dados
			}
		}
		d.mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	t.Fatalf("o evento %s com %s=%s não chegou; chegaram %d eventos, %d com assinatura inválida", tipo, campo, valor, len(d.eventos), d.invalidos)
	return nil
}

// sandbox é um `vintem sandbox` rodando num diretório próprio.
type sandbox struct {
	cmd   *exec.Cmd
	dir   string
	url   string
	token string
	saida *registro
	// fim fecha quando o processo termina; erroFim é o que o Wait devolveu.
	fim     chan struct{}
	erroFim error
}

// registro guarda o stderr do processo; o exec escreve enquanto o teste lê.
type registro struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (r *registro) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.b.Write(p)
}

func (r *registro) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.b.String()
}

func portaLivre(t *testing.T) string {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func subirSandbox(t *testing.T) *sandbox {
	t.Helper()
	addr := portaLivre(t)
	s := &sandbox{dir: t.TempDir(), url: "http://" + addr, saida: &registro{}, fim: make(chan struct{})}
	s.cmd = exec.CommandContext(context.Background(), binario, "sandbox") //nolint:gosec // G204: binário da própria suíte
	s.cmd.Dir = s.dir
	s.cmd.Env = append(os.Environ(),
		"VINTEM_ADDR="+addr,
		// O pagamento do sandbox leva 5 s de relógio; o resto anda rápido.
		"VINTEM_CONCILIACAO_INTERVALO=500ms",
		"VINTEM_WEBHOOK_INTERVALO=200ms",
		"VINTEM_CERTIFICADO_INTERVALO=0",
	)
	stdout, err := s.cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	s.cmd.Stderr = s.saida
	if err := s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	tokens := make(chan string, 1)
	go func() {
		re := regexp.MustCompile(`API key\s+(vtm_\S+)`)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if m := re.FindStringSubmatch(sc.Text()); m != nil {
				tokens <- m[1]
			}
		}
	}()
	go func() {
		s.erroFim = s.cmd.Wait()
		close(s.fim)
	}()
	t.Cleanup(func() {
		select {
		case <-s.fim:
		default:
			_ = s.cmd.Process.Kill()
			<-s.fim
		}
	})

	select {
	case s.token = <-tokens:
	case <-s.fim:
		t.Fatalf("o sandbox terminou antes de subir: %v\n%s", s.erroFim, s.saida)
	case <-time.After(30 * time.Second):
		t.Fatalf("o sandbox não imprimiu a API key\n%s", s.saida)
	}
	limite := time.Now().Add(15 * time.Second)
	for {
		if resp, err := http.Get(s.url + "/health"); err == nil { //nolint:noctx // espera curta de teste
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return s
			}
		}
		if time.Now().After(limite) {
			t.Fatalf("o sandbox não respondeu /health\n%s", s.saida)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// cli roda outro comando do mesmo binário no banco do sandbox, como o operador faria.
func (s *sandbox) cli(t *testing.T, args ...string) string {
	t.Helper()
	chave, err := os.ReadFile(filepath.Join(s.dir, "vintem-sandbox.key"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), binario, args...) //nolint:gosec // G204: binário da própria suíte
	cmd.Dir = s.dir
	cmd.Env = append(os.Environ(), "VINTEM_DB=vintem-sandbox.db", "VINTEM_MASTER_KEY="+strings.TrimSpace(string(chave)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("vintem %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// api chama a API com a key do sandbox e decodifica a resposta em saida.
func (s *sandbox) api(t *testing.T, metodo, caminho, corpo string, cabecalhos map[string]string, saida any) int {
	t.Helper()
	var leitor io.Reader
	if corpo != "" {
		leitor = strings.NewReader(corpo)
	}
	req, err := http.NewRequestWithContext(t.Context(), metodo, s.url+caminho, leitor)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	if corpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if saida != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(b, saida); err != nil {
			t.Fatalf("%s %s: resposta fora do formato: %v\n%s", metodo, caminho, err, b)
		}
	}
	return resp.StatusCode
}

// desligar manda SIGINT, como o Ctrl+C ou o docker stop, e espera sair limpo.
func (s *sandbox) desligar(t *testing.T) {
	t.Helper()
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.fim:
		if s.erroFim != nil {
			t.Fatalf("o sandbox saiu com erro: %v\n%s", s.erroFim, s.saida)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("o sandbox não desligou em 15 s\n%s", s.saida)
	}
}

// TestCobrancaPagaChegaAssinadaNoDestino é o fluxo do docs/ARQUITETURA.md: o
// binário em modo sandbox cria a cobrança, o "pagador" paga, e os eventos chegam
// num servidor de teste com assinatura válida. Depois, a devolução faz o mesmo
// caminho, e o processo desliga limpo.
func TestCobrancaPagaChegaAssinadaNoDestino(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e: sobe o binário e espera o pagamento do sandbox (5 s)")
	}
	s := subirSandbox(t)
	d := novoDestino(t)

	saida := s.cli(t, "destino", "criar", "--url", d.srv.URL+"/webhooks/vintem")
	whsec := regexp.MustCompile(`whsec_\S+`).FindString(saida)
	if whsec == "" {
		t.Fatalf("o segredo do destino não apareceu:\n%s", saida)
	}
	d.mu.Lock()
	d.whsec = whsec
	d.mu.Unlock()

	var cob struct {
		Txid          string `json:"txid"`
		Status        string `json:"status"`
		PixCopiaECola string `json:"pixCopiaECola"`
	}
	corpo := `{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@vintem.dev","solicitacaoPagador":"Pedido 42"}`
	if st := s.api(t, http.MethodPost, "/cob", corpo, map[string]string{"Idempotency-Key": "e2e-pedido-42"}, &cob); st != http.StatusCreated {
		t.Fatalf("criar cob: %d", st)
	}
	if cob.Status != "ATIVA" || !strings.HasPrefix(cob.PixCopiaECola, "000201") {
		t.Fatalf("cob criada: %+v", cob)
	}
	d.esperar(t, "cobranca.criada", "txid", cob.Txid)

	// O sandbox paga em 5 s; a conciliação grava o Pix e conclui a cobrança.
	pix := d.esperar(t, "pix.recebido", "txid", cob.Txid)
	if pix["valor"] != "49.90" {
		t.Fatalf("pix.recebido: %v", pix)
	}
	d.esperar(t, "cobranca.concluida", "txid", cob.Txid)
	e2e, _ := pix["endToEndId"].(string)

	var paga struct {
		Status string `json:"status"`
		Pix    []struct {
			EndToEndID string `json:"endToEndId"`
		} `json:"pix"`
	}
	if st := s.api(t, http.MethodGet, "/cob/"+cob.Txid, "", nil, &paga); st != http.StatusOK || paga.Status != "CONCLUIDA" || len(paga.Pix) != 1 || paga.Pix[0].EndToEndID != e2e {
		t.Fatalf("consultar a cob paga: %d %+v", st, paga)
	}

	if st := s.api(t, http.MethodPut, "/pix/"+e2e+"/devolucao/e2e1", `{"valor":"9.90","descricao":"Item com defeito"}`, nil, nil); st != http.StatusCreated {
		t.Fatalf("devolver: %d", st)
	}
	d.esperar(t, "devolucao.solicitada", "id", "e2e1")

	s.desligar(t)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.invalidos != 0 || d.repetidos != 0 {
		t.Fatalf("%d eventos com assinatura inválida, %d entregues duas vezes", d.invalidos, d.repetidos)
	}
}
