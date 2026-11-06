package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/store/sqlite/db"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

type pixes struct{ q *db.Queries }

func (r pixes) Registrar(ctx context.Context, p domain.PixRecebido) (bool, error) {
	doc, nome := devedor(p.Pagador)
	n, err := r.q.RegistrarPix(ctx, db.RegistrarPixParams{
		EndToEndID: p.EndToEndID, ContaID: p.ContaID, Txid: string(p.Txid), Valor: int64(p.Valor), Chave: p.Chave,
		Horario: p.Horario.UnixNano(), InfoPagador: p.InfoPagador, PagadorDocumento: doc, PagadorNome: nome,
	})
	if chaveEstrangeira(err) {
		return false, fmt.Errorf("conta %s: %w", p.ContaID, ports.ErrNaoEncontrado)
	}
	if err != nil {
		return false, traduzir(err, "pix "+p.EndToEndID)
	}
	return n == 1, nil
}

func (r pixes) BuscarPorE2E(ctx context.Context, e2e string) (domain.PixRecebido, error) {
	row, err := r.q.BuscarPixPorE2E(ctx, e2e)
	if err != nil {
		return domain.PixRecebido{}, traduzir(err, "pix "+e2e)
	}
	return paraPix(row), nil
}

func (r pixes) ListarPorTxid(ctx context.Context, txid domain.Txid) ([]domain.PixRecebido, error) {
	rows, err := r.q.ListarPixPorTxid(ctx, string(txid))
	if err != nil {
		return nil, traduzir(err, "pix da cobrança "+string(txid))
	}
	out := make([]domain.PixRecebido, len(rows))
	for i, row := range rows {
		out[i] = paraPix(row)
	}
	return out, nil
}

func (r pixes) Listar(ctx context.Context, f ports.FiltroPix) ([]domain.PixRecebido, error) {
	limite := f.Limite
	if limite <= 0 {
		limite = 100
	}
	p := filtrosPix(f)
	rows, err := r.q.ListarPix(ctx, db.ListarPixParams{
		ContaID: p.ContaID, Inicio: p.Inicio, Fim: p.Fim, Txid: p.Txid, TxidPresente: p.TxidPresente,
		DevolucaoPresente: p.DevolucaoPresente, Documento: p.Documento, Limite: int64(limite), Pular: int64(f.Pular),
	})
	if err != nil {
		return nil, traduzir(err, "pix recebidos")
	}
	out := make([]domain.PixRecebido, len(rows))
	for i, row := range rows {
		out[i] = paraPix(row)
	}
	return out, nil
}

func (r pixes) Contar(ctx context.Context, f ports.FiltroPix) (int, error) {
	n, err := r.q.ContarPix(ctx, filtrosPix(f))
	if err != nil {
		return 0, traduzir(err, "pix recebidos")
	}
	return int(n), nil
}

// filtrosPix converte o filtro do port nos parâmetros opcionais do sqlc (nil = sem filtro).
func filtrosPix(f ports.FiltroPix) db.ContarPixParams {
	var p db.ContarPixParams
	if f.ContaID != "" {
		p.ContaID = f.ContaID
	}
	if !f.Janela.Inicio.IsZero() {
		p.Inicio = f.Janela.Inicio.UnixNano()
	}
	if !f.Janela.Fim.IsZero() {
		p.Fim = f.Janela.Fim.UnixNano()
	}
	if f.Txid != "" {
		p.Txid = string(f.Txid)
	}
	if f.TxidPresente != nil {
		p.TxidPresente = *f.TxidPresente
	}
	if f.DevolucaoPresente != nil {
		p.DevolucaoPresente = *f.DevolucaoPresente
	}
	if f.Documento != "" {
		p.Documento = f.Documento
	}
	return p
}

func paraPix(r db.PixRecebido) domain.PixRecebido {
	p := domain.PixRecebido{
		EndToEndID: r.EndToEndID, ContaID: r.ContaID, Txid: domain.Txid(r.Txid), Valor: domain.Money(r.Valor),
		Chave: r.Chave, Horario: time.Unix(0, r.Horario).UTC(), InfoPagador: r.InfoPagador,
	}
	if r.PagadorDocumento.Valid {
		p.Pagador = &domain.Devedor{Documento: r.PagadorDocumento.String, Nome: r.PagadorNome.String}
	}
	return p
}

func (r contas) SalvarTokenWebhook(ctx context.Context, id string, hash []byte) error {
	n, err := r.q.SalvarTokenWebhook(ctx, db.SalvarTokenWebhookParams{WebhookTokenHash: hash, ID: id})
	if err != nil {
		return traduzir(err, "webhook da conta "+id)
	}
	if n == 0 {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r contas) BuscarTokenWebhook(ctx context.Context, id string) ([]byte, error) {
	h, err := r.q.BuscarTokenWebhook(ctx, id)
	if err != nil {
		return nil, traduzir(err, "conta "+id)
	}
	if h == nil {
		return nil, fmt.Errorf("webhook da conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return h, nil
}
