package notifier

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// destino é a aplicação do cliente: guarda a última requisição e responde com o handler dado.
func destino(t *testing.T, responder http.HandlerFunc) (*httptest.Server, *atomic.Pointer[http.Request], *atomic.Pointer[[]byte]) {
	t.Helper()
	var req atomic.Pointer[http.Request]
	var corpo atomic.Pointer[[]byte]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		req.Store(r.Clone(context.Background()))
		corpo.Store(&b)
		responder(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &req, &corpo
}

func TestEnviarFazOPOSTComCorpoECabecalhos(t *testing.T) {
	srv, req, corpo := destino(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	evento := []byte(`{"type":"pix.recebido","data":{"valor":"10.00"}}`)
	cab := map[string]string{"webhook-id": "evt_1", "webhook-timestamp": "1761739200", "webhook-signature": "v1,abc="}

	status, err := NovoHTTP("v1.2.3").Enviar(context.Background(), srv.URL+"/webhooks/vintem", cab, evento)
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("Enviar = %d, %v", status, err)
	}
	r := req.Load()
	if r.Method != http.MethodPost || r.URL.Path != "/webhooks/vintem" {
		t.Fatalf("requisição: %s %s", r.Method, r.URL.Path)
	}
	// O corpo vai byte a byte como foi assinado: qualquer mudança quebra a assinatura.
	if string(*corpo.Load()) != string(evento) {
		t.Fatalf("corpo: %s", *corpo.Load())
	}
	if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("User-Agent") != "Vintém/v1.2.3" {
		t.Fatalf("Content-Type %q, User-Agent %q", r.Header.Get("Content-Type"), r.Header.Get("User-Agent"))
	}
	for k, v := range cab {
		if r.Header.Get(k) != v {
			t.Errorf("%s = %q, quer %q", k, r.Header.Get(k), v)
		}
	}
}

// O status do destino volta como veio: quem decide se foi entrega ou falha é a Saida.
func TestEnviarDevolveOStatusDoDestino(t *testing.T) {
	for _, quer := range []int{http.StatusOK, http.StatusAccepted, http.StatusBadRequest, http.StatusGone, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		srv, _, _ := destino(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(quer) })
		if status, err := NovoHTTP("t").Enviar(context.Background(), srv.URL, nil, []byte(`{}`)); err != nil || status != quer {
			t.Errorf("destino respondeu %d: Enviar = %d, %v", quer, status, err)
		}
	}
}

// Redirecionamento não é seguido: o evento não pode parar numa URL que ninguém
// configurou, e o 3xx aparece como falha em vez de entrega silenciosa.
func TestEnviarNaoSegueRedirecionamento(t *testing.T) {
	var desviado atomic.Bool
	outro := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		desviado.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(outro.Close)
	for _, codigo := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		srv, _, _ := destino(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, outro.URL, codigo) })
		status, err := NovoHTTP("t").Enviar(context.Background(), srv.URL, nil, []byte(`{}`))
		if err != nil || status != codigo {
			t.Errorf("%d: Enviar = %d, %v", codigo, status, err)
		}
	}
	if desviado.Load() {
		t.Fatal("o evento foi entregue na URL do redirecionamento")
	}
}

func TestEnviarDestinoLentoViraTimeout(t *testing.T) {
	solta := make(chan struct{})
	srv, _, _ := destino(t, func(w http.ResponseWriter, _ *http.Request) {
		<-solta
		w.WriteHeader(http.StatusOK)
	})
	defer close(solta)
	h := NovoHTTP("t")
	if h.cliente.Timeout != 10*time.Second {
		t.Fatalf("timeout = %v, a mensagem de erro diz 10 s", h.cliente.Timeout)
	}
	h.cliente.Timeout = 50 * time.Millisecond
	status, err := h.Enviar(context.Background(), srv.URL, nil, []byte(`{}`))
	if err == nil || status != 0 || err.Error() != "sem resposta em 10 s" {
		t.Fatalf("Enviar = %d, %v", status, err)
	}
}

func TestEnviarSemConexao(t *testing.T) {
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endereco := "http://" + l.Addr().String()
	_ = l.Close() // ninguém escuta mais nessa porta
	status, err := NovoHTTP("t").Enviar(context.Background(), endereco, nil, []byte(`{}`))
	if err == nil || status != 0 || strings.Contains(err.Error(), "10 s") {
		t.Fatalf("Enviar = %d, %v", status, err)
	}
}

// O contexto cancelado (o serve desligando) interrompe a entrega em andamento.
func TestEnviarRespeitaOContexto(t *testing.T) {
	solta := make(chan struct{})
	srv, _, _ := destino(t, func(w http.ResponseWriter, _ *http.Request) {
		<-solta
		w.WriteHeader(http.StatusOK)
	})
	defer close(solta)
	ctx, cancelar := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancelar)
	inicio := time.Now()
	_, err := NovoHTTP("t").Enviar(ctx, srv.URL, nil, []byte(`{}`))
	if !errors.Is(err, context.Canceled) || time.Since(inicio) > 5*time.Second {
		t.Fatalf("Enviar = %v depois de %v", err, time.Since(inicio))
	}
}

func TestEnviarURLInvalida(t *testing.T) {
	if _, err := NovoHTTP("t").Enviar(context.Background(), "://sem-esquema", nil, nil); err == nil || !strings.Contains(err.Error(), "montando requisição") {
		t.Fatalf("err = %v", err)
	}
}

// Resposta grande não é lida inteira: o destino não consegue prender a fila
// mandando um corpo sem fim depois do status.
func TestEnviarNaoLeRespostaSemFim(t *testing.T) {
	srv, _, _ := destino(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		bloco := []byte(strings.Repeat("x", 32<<10))
		for r.Context().Err() == nil {
			if _, err := w.Write(bloco); err != nil {
				return
			}
		}
	})
	feito := make(chan struct{})
	go func() {
		defer close(feito)
		if status, err := NovoHTTP("t").Enviar(context.Background(), srv.URL, nil, []byte(`{}`)); err != nil || status != http.StatusOK {
			t.Errorf("Enviar = %d, %v", status, err)
		}
	}()
	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal("Enviar ficou lendo a resposta sem fim")
	}
}
