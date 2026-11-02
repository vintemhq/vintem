package httpapi

import (
	"context"
	"fmt"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// errSemRecebimentos responde 501 quando o servidor foi montado sem os Pix recebidos.
var errSemRecebimentos = fmt.Errorf("%w: os Pix recebidos não estão configurados nesta instância", ports.ErrNaoSuportado)

// ListarPix lista os Pix gravados na janela, paginados como a spec.
func (s *Server) ListarPix(ctx context.Context, r gen.ListarPixRequestObject) (gen.ListarPixResponseObject, error) {
	if s.webhooks.Recebimentos.Store == nil {
		return nil, errSemRecebimentos
	}
	q := app.ConsultaPix{Inicio: r.Params.Inicio, Fim: r.Params.Fim, TxidPresente: r.Params.TxIdPresente, DevolucaoPresente: r.Params.DevolucaoPresente}
	if r.Params.Txid != nil {
		q.Txid = domain.Txid(*r.Params.Txid)
	}
	if r.Params.Cpf != nil {
		q.Documento = *r.Params.Cpf
	}
	if r.Params.Cnpj != nil {
		q.Documento = *r.Params.Cnpj
	}
	if r.Params.PaginacaoPaginaAtual != nil {
		q.Pagina = int(*r.Params.PaginacaoPaginaAtual)
	}
	if r.Params.PaginacaoItensPorPagina != nil {
		q.ItensPagina = int(*r.Params.PaginacaoItensPorPagina)
	}
	pag, err := s.webhooks.Recebimentos.Listar(ctx, q)
	if err != nil {
		return nil, err
	}
	pix := make([]gen.Pix, len(pag.Pix))
	for i, p := range pag.Pix {
		pix[i] = pixGen(p)
	}
	return gen.ListarPix200JSONResponse{
		Pix: pix,
		Parametros: gen.ParametrosConsultaPix{
			Inicio: q.Inicio, Fim: q.Fim, Txid: r.Params.Txid, TxIdPresente: r.Params.TxIdPresente, DevolucaoPresente: r.Params.DevolucaoPresente,
			Cpf: r.Params.Cpf, Cnpj: r.Params.Cnpj,
			Paginacao: gen.Paginacao{PaginaAtual: pag.Pagina, ItensPorPagina: pag.ItensPagina, QuantidadeDePaginas: pag.TotalPaginas, QuantidadeTotalDeItens: pag.TotalItens},
		},
	}, nil
}

// ConsultarPix devolve o Pix gravado, se a API key enxergar a conta dele.
func (s *Server) ConsultarPix(ctx context.Context, r gen.ConsultarPixRequestObject) (gen.ConsultarPixResponseObject, error) {
	if s.webhooks.Recebimentos.Store == nil {
		return nil, errSemRecebimentos
	}
	p, err := s.webhooks.Recebimentos.Consultar(ctx, r.E2eid)
	if err != nil {
		return nil, err
	}
	return gen.ConsultarPix200JSONResponse(pixGen(p)), nil
}

// pixGen é o Pix da spec, com as devoluções quando houver.
func pixGen(p domain.PixRecebido) gen.Pix {
	g := gen.Pix{EndToEndId: p.EndToEndID, Txid: vazioNil(string(p.Txid)), Valor: p.Valor.String(), Horario: p.Horario, Chave: vazioNil(p.Chave), InfoPagador: vazioNil(p.InfoPagador)}
	if len(p.Devolucoes) > 0 {
		devs := make([]gen.Devolucao, len(p.Devolucoes))
		for i, d := range p.Devolucoes {
			devs[i] = devolucaoGen(d)
		}
		g.Devolucoes = &devs
	}
	return g
}
