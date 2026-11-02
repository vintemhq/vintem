-- name: RegistrarPix :execrows
INSERT INTO pix_recebidos (end_to_end_id, conta_id, txid, valor, chave, horario, info_pagador, pagador_documento, pagador_nome)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (end_to_end_id) DO NOTHING;

-- name: BuscarPixPorE2E :one
SELECT * FROM pix_recebidos WHERE end_to_end_id = ?;

-- name: ListarPixPorTxid :many
SELECT * FROM pix_recebidos WHERE txid = ? ORDER BY horario, end_to_end_id;

-- name: ListarPix :many
-- Os filtros do GET /pix; NULL desliga o filtro.
SELECT * FROM pix_recebidos
WHERE (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(inicio) IS NULL OR horario >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR horario < sqlc.narg(fim))
  AND (sqlc.narg(txid) IS NULL OR txid = sqlc.narg(txid))
  AND (sqlc.narg(txid_presente) IS NULL OR (txid <> '') = sqlc.narg(txid_presente))
  AND (sqlc.narg(devolucao_presente) IS NULL OR EXISTS (SELECT 1 FROM devolucoes d WHERE d.end_to_end_id = pix_recebidos.end_to_end_id) = sqlc.narg(devolucao_presente))
  AND (sqlc.narg(documento) IS NULL OR pagador_documento = sqlc.narg(documento))
ORDER BY horario, end_to_end_id
LIMIT @limite OFFSET @pular;

-- name: ContarPix :one
SELECT COUNT(*) FROM pix_recebidos
WHERE (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(inicio) IS NULL OR horario >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR horario < sqlc.narg(fim))
  AND (sqlc.narg(txid) IS NULL OR txid = sqlc.narg(txid))
  AND (sqlc.narg(txid_presente) IS NULL OR (txid <> '') = sqlc.narg(txid_presente))
  AND (sqlc.narg(devolucao_presente) IS NULL OR EXISTS (SELECT 1 FROM devolucoes d WHERE d.end_to_end_id = pix_recebidos.end_to_end_id) = sqlc.narg(devolucao_presente))
  AND (sqlc.narg(documento) IS NULL OR pagador_documento = sqlc.narg(documento));
