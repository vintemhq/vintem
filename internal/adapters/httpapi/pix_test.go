package httpapi

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/domain"
)

// janelaPix monta o GET /pix com a janela de uma hora em volta de agora e os filtros extras.
func (a *api) janelaPix(extras url.Values) string {
	q := url.Values{"inicio": {a.clock.Now().Add(-time.Hour).Format(time.RFC3339)}, "fim": {a.clock.Now().Add(time.Hour).Format(time.RFC3339)}}
	for k, v := range extras {
		q[k] = v
	}
	return "/pix?" + q.Encode()
}

func TestListarEConsultarPix(t *testing.T) {
	a := novaAPI(t)
	e2e := a.pixPago()
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"3.45"}`, nil, nil); st != http.StatusCreated {
		t.Fatalf("devolver: %d", st)
	}

	var pag gen.PixConsultados
	if st := a.chamar(http.MethodGet, a.janelaPix(nil), "", nil, &pag); st != http.StatusOK {
		t.Fatalf("listar: %d", st)
	}
	if len(pag.Pix) != 1 || pag.Pix[0].EndToEndId != e2e || pag.Pix[0].Valor != "123.45" || deref(pag.Pix[0].Txid) != txid ||
		pag.Parametros.Paginacao.QuantidadeTotalDeItens != 1 || pag.Parametros.Paginacao.ItensPorPagina != 100 {
		t.Fatalf("listar = %+v", pag)
	}
	if pag.Pix[0].Devolucoes == nil || len(*pag.Pix[0].Devolucoes) != 1 || (*pag.Pix[0].Devolucoes)[0].Id != "dev1" {
		t.Fatalf("devoluções na lista = %+v", pag.Pix[0].Devolucoes)
	}

	var p gen.Pix
	if st := a.chamar(http.MethodGet, "/pix/"+e2e, "", nil, &p); st != http.StatusOK || p.EndToEndId != e2e || p.Devolucoes == nil || (*p.Devolucoes)[0].Valor != "3.45" {
		t.Fatalf("consultar: %d %+v", st, p)
	}

	casos := []struct {
		nome    string
		filtros url.Values
		quer    int
	}{
		{"com txid", url.Values{"txIdPresente": {"true"}}, 1},
		{"sem txid", url.Values{"txIdPresente": {"false"}}, 0},
		{"pelo txid", url.Values{"txid": {txid}}, 1},
		{"com devolução", url.Values{"devolucaoPresente": {"true"}}, 1},
		{"sem devolução", url.Values{"devolucaoPresente": {"false"}}, 0},
		{"segunda página", url.Values{"paginacao.paginaAtual": {"1"}, "paginacao.itensPorPagina": {"1"}}, 0},
	}
	for _, c := range casos {
		var pag gen.PixConsultados
		if st := a.chamar(http.MethodGet, a.janelaPix(c.filtros), "", nil, &pag); st != http.StatusOK || len(pag.Pix) != c.quer {
			t.Errorf("%s: %d, %d Pix, quer %d", c.nome, st, len(pag.Pix), c.quer)
		}
	}
}

func TestListarPixRecusa(t *testing.T) {
	a := novaAPI(t)
	agora := a.clock.Now().Format(time.RFC3339)
	casos := []struct {
		nome, caminho string
		status        int
	}{
		{"sem janela", "/pix", http.StatusBadRequest},
		{"fim antes do inicio", "/pix?" + url.Values{"inicio": {agora}, "fim": {a.clock.Now().Add(-time.Hour).Format(time.RFC3339)}}.Encode(), http.StatusBadRequest},
		{"itens demais", a.janelaPix(url.Values{"paginacao.itensPorPagina": {"1001"}}), http.StatusBadRequest},
		{"pix desconhecido", "/pix/E00000000202610291200naoexiste01", http.StatusNotFound},
		{"endToEndId fora do formato", "/pix/curto", http.StatusBadRequest},
	}
	for _, c := range casos {
		if st := a.chamar(http.MethodGet, c.caminho, "", nil, nil); st != c.status {
			t.Errorf("%s: %d, quer %d", c.nome, st, c.status)
		}
	}
}

func TestPixRespeitaEscopoEConta(t *testing.T) {
	a := novaAPI(t)
	e2e := a.pixPago()
	h := func(token string) map[string]string { return map[string]string{"Authorization": "Bearer " + token} }

	if st := a.chamar(http.MethodGet, "/pix/"+e2e, "", h(a.novoToken("loja", domain.EscopoCobRead)), nil); st != http.StatusForbidden {
		t.Fatalf("sem pix:read: %d", st)
	}
	if st := a.chamar(http.MethodGet, "/pix/"+e2e, "", h(a.novoToken("loja", domain.EscopoPixRead)), nil); st != http.StatusOK {
		t.Fatalf("com pix:read da conta: %d", st)
	}
	if err := a.store.Contas().Criar(a.t.Context(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	daOutra := h(a.novoToken("outra", domain.EscopoPixRead))
	if st := a.chamar(http.MethodGet, "/pix/"+e2e, "", daOutra, nil); st != http.StatusNotFound {
		t.Fatalf("consultar Pix de outra conta: %d", st)
	}
	var pag gen.PixConsultados
	if st := a.chamar(http.MethodGet, a.janelaPix(nil), "", daOutra, &pag); st != http.StatusOK || len(pag.Pix) != 0 || pag.Parametros.Paginacao.QuantidadeTotalDeItens != 0 {
		t.Fatalf("listar com key de outra conta: %d %+v", st, pag)
	}
}
