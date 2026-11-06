package pixapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vintemhq/vintem/internal/domain"
)

// LimiteResposta protege contra resposta gigante (bug ou ataque no caminho).
const LimiteResposta = 4 << 20

// ErrDevolucaoDuplicada é o que o Traduzir do banco devolve para id de
// devolução repetido. Não sai da API: o Devolver a transforma em idempotência.
var ErrDevolucaoDuplicada = errors.New("devolução com esse id já existe")

// ErroHTTP guarda o status da resposta de erro, para quem precisa decidir por ele.
type ErroHTTP struct {
	Status int
	Err    error
}

func (e *ErroHTTP) Error() string { return e.Err.Error() }
func (e *ErroHTTP) Unwrap() error { return e.Err }

// chamar faz a requisição na API Pix da conta e decodifica a resposta em saida.
func (a *API) chamar(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any) error {
	return a.chamarCom(ctx, conta, metodo, caminho, entrada, saida, nil)
}

// chamarCom é o chamar com headers extras.
func (a *API) chamarCom(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any, headers map[string]string) error {
	c, err := a.clienteDa(ctx, conta)
	if err != nil {
		return err
	}
	return a.chamarEm(ctx, c, metodo, c.base+caminho, entrada, saida, headers)
}

// chamarEm faz a requisição no endereço completo, com o token e os
// parâmetros da conta, e decodifica a resposta em saida.
func (a *API) chamarEm(ctx context.Context, c *cliente, metodo, endereco string, entrada, saida any, headers map[string]string) error {
	nome := a.cfg.Nome
	caminho := strings.TrimPrefix(endereco, c.base)
	var corpo io.Reader
	if entrada != nil {
		b, err := json.Marshal(entrada)
		if err != nil {
			return fmt.Errorf("%s: montando %s %s: %w", nome, metodo, caminho, err)
		}
		// bytes.Reader permite ao oauth.Transport reenviar o corpo depois de um 401.
		corpo = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, endereco, corpo)
	if err != nil {
		return err
	}
	if len(c.parametros) > 0 {
		q := req.URL.Query()
		for k, v := range c.parametros {
			q[k] = v
		}
		req.URL.RawQuery = q.Encode()
	}
	if entrada != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if a.cfg.HeaderClientID != "" {
		req.Header.Set(a.cfg.HeaderClientID, c.clientID)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s %s: %w", nome, metodo, caminho, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, LimiteResposta))
	if err != nil {
		return fmt.Errorf("%s: lendo resposta de %s %s: %w", nome, metodo, caminho, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: %s %s: %w", nome, metodo, caminho, &ErroHTTP{Status: resp.StatusCode, Err: a.cfg.Traduzir(resp.StatusCode, b)})
	}
	if saida == nil {
		return nil
	}
	if err := json.Unmarshal(b, saida); err != nil {
		return fmt.Errorf("%s: resposta de %s %s fora do formato: %w", nome, metodo, caminho, err)
	}
	return nil
}

// ConfigurarWebhook registra a URL de aviso na chave Pix da conta
// (PUT {prefixo}/webhook/:chave), com os headers que o banco pedir.
func (a *API) ConfigurarWebhook(ctx context.Context, conta domain.Conta, endereco string) error {
	corpo := map[string]string{"webhookUrl": endereco}
	return a.chamarCom(ctx, conta, http.MethodPut, a.cfg.Prefixo+"/webhook/"+escapar(conta.ChavePix), corpo, nil, a.cfg.HeadersWebhook)
}

// RemoverWebhook cancela a URL de aviso da chave Pix da conta
// (DELETE {prefixo}/webhook/:chave).
func (a *API) RemoverWebhook(ctx context.Context, conta domain.Conta) error {
	return a.chamar(ctx, conta, http.MethodDelete, a.cfg.Prefixo+"/webhook/"+escapar(conta.ChavePix), nil, nil)
}

// ChamarNoHost chama um caminho no mesmo host da API Pix da conta, fora da
// base (o simulador de pagamento da homologação do BB), com o token e os
// parâmetros da conta.
func (a *API) ChamarNoHost(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any) error {
	c, err := a.clienteDa(ctx, conta)
	if err != nil {
		return err
	}
	u, err := url.Parse(c.base)
	if err != nil {
		return fmt.Errorf("%s: URL da API: %w", a.cfg.Nome, err)
	}
	return a.chamarEm(ctx, c, metodo, u.Scheme+"://"+u.Host+caminho, entrada, saida, nil)
}

// Chamar é a chamada genérica, para a rota que só um banco tem (por exemplo,
// a simulação de pagamento do sandbox do Inter). caminho já inclui o prefixo.
func (a *API) Chamar(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any) error {
	return a.chamar(ctx, conta, metodo, caminho, entrada, saida)
}
