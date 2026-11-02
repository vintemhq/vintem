// Contra um `vintem sandbox` de verdade: defina VINTEM_URL e VINTEM_TOKEN.
import { test } from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Vintem, VintemError } from "../dist/index.js";

const { VINTEM_URL: url, VINTEM_TOKEN: token } = process.env;
const pular = !url || !token ? "defina VINTEM_URL e VINTEM_TOKEN (vintem sandbox)" : false;
const vintem = pular ? null : new Vintem({ url, token });

test("o exemplo do README cria uma cobrança", { skip: pular }, async () => {
  const { stdout } = await promisify(execFile)(process.execPath, [new URL("./exemplo.mjs", import.meta.url).pathname], { env: process.env });
  assert.match(stdout, /^\w{26,35} ATIVA 0002/);
});

test("lista e consulta Pix recebidos", { skip: pular }, async () => {
  const lista = await vintem.pix.listar({ inicio: new Date(Date.now() - 86_400_000).toISOString(), fim: new Date(Date.now() + 60_000).toISOString() });
  assert.ok(Array.isArray(lista.pix));
  assert.equal(lista.parametros.paginacao.itensPorPagina, 100);
  for (const p of lista.pix) assert.equal((await vintem.pix.consultar(p.endToEndId)).endToEndId, p.endToEndId);
  await assert.rejects(vintem.pix.consultar("E00000000202610291200naoexiste01"), (e) => e instanceof VintemError && e.status === 404);
});

test("cria, consulta, lista e cancela", { skip: pular }, async () => {
  const inicio = new Date(Date.now() - 60_000).toISOString();
  const cob = await vintem.cob.criar({ calendario: { expiracao: 3600 }, valor: { original: "15.00" }, chave: "sandbox@vintem.dev" }, { idempotencyKey: "sdk-" + Date.now() });
  assert.equal(cob.status, "ATIVA");
  assert.equal((await vintem.cob.consultar(cob.txid)).txid, cob.txid);
  const lista = await vintem.cob.listar({ inicio, fim: new Date(Date.now() + 60_000).toISOString() });
  assert.ok(lista.cobs.some((c) => c.txid === cob.txid));
  assert.equal((await vintem.cob.cancelar(cob.txid)).status, "REMOVIDA_PELO_USUARIO_RECEBEDOR");
});

test("a mesma chave de idempotência não cria duas cobranças", { skip: pular }, async () => {
  const corpo = { calendario: { expiracao: 3600 }, valor: { original: "16.00" }, chave: "sandbox@vintem.dev" };
  const chave = "sdk-idem-" + Date.now();
  const a = await vintem.cob.criar(corpo, { idempotencyKey: chave });
  const b = await vintem.cob.criar(corpo, { idempotencyKey: chave });
  assert.equal(a.txid, b.txid);
});

test("erro da API vira VintemError com o problema", { skip: pular }, async () => {
  await assert.rejects(vintem.cob.consultar("txidquenaoexiste000000000001"), (e) => e instanceof VintemError && e.status === 404 && typeof e.problema?.title === "string");
  const semEscopo = new Vintem({ url, token: "vtm_tokeninvalido" });
  await assert.rejects(semEscopo.cob.consultar("x"), (e) => e instanceof VintemError && e.status === 401);
});
