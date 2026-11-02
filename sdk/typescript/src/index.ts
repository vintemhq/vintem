// SDK TypeScript do Vintém. Os tipos vêm de ./schema.ts, gerado do
// api/openapi.yaml (npm run gerar); este arquivo só liga cada operação à rota.
import type { components, operations } from "./schema.js";

export type { components, operations };
type Schemas = components["schemas"];
type Corpo<O extends keyof operations> = operations[O] extends { requestBody: { content: { "application/json": infer C } } } ? C : never;
type Resposta<O extends keyof operations, S extends number> = operations[O] extends {
  responses: { [K in S]: { content: { "application/json": infer R } } };
}
  ? R
  : never;
type Consulta<O extends keyof operations> = operations[O] extends { parameters: { query?: infer Q } } ? NonNullable<Q> : never;

export type Cobranca = Schemas["CobCompleta"];
export type CobrancaVencimento = Schemas["CobVCompleta"];
export type Devolucao = Schemas["Devolucao"];
export type Pix = Schemas["Pix"];
export type Problema = Schemas["Problema"];

export interface OpcoesVintem {
  /** URL da API do Vintém, ex.: http://localhost:8080 */
  url?: string;
  /** API key (vtm_...). Os escopos da key decidem o que o cliente pode fazer. */
  token: string;
  /** fetch alternativo (testes, proxies). Padrão: o fetch global. */
  fetch?: typeof fetch;
}

/** Erro da API do Vintém, com o problema no formato da spec do BCB (RFC 7807). */
export class VintemError extends Error {
  constructor(
    readonly status: number,
    readonly problema: Problema | undefined,
    corpo: string,
  ) {
    super(problema?.detail ?? problema?.title ?? `o Vintém respondeu ${status}: ${corpo}`);
    this.name = "VintemError";
  }
}

export class Vintem {
  readonly url: string;
  private readonly token: string;
  private readonly fetch: typeof fetch;

  constructor(opcoes: OpcoesVintem) {
    if (!opcoes.token) throw new Error("informe o token (API key vtm_...) do Vintém");
    this.url = (opcoes.url ?? "http://localhost:8080").replace(/\/+$/, "");
    this.token = opcoes.token;
    this.fetch = opcoes.fetch ?? globalThis.fetch;
  }

  /** Cobrança imediata (cob). */
  readonly cob = {
    /** Cria com txid gerado pelo Vintém. A chave de idempotência evita cobrança dupla ao tentar de novo. */
    criar: (corpo: Corpo<"criarCob">, opcoes?: { idempotencyKey?: string }) =>
      this.chamar<Resposta<"criarCob", 201>>("POST", "/cob", corpo, { "Idempotency-Key": opcoes?.idempotencyKey ?? novoId() }),
    /** Cria com o txid do seu sistema (26 a 35 caracteres alfanuméricos). */
    criarComTxid: (txid: string, corpo: Corpo<"criarCobComTxid">) =>
      this.chamar<Resposta<"criarCobComTxid", 201>>("PUT", `/cob/${encodeURIComponent(txid)}`, corpo),
    /** Consulta; se ainda estiver ATIVA, o Vintém pergunta ao banco se já foi paga. */
    consultar: (txid: string) => this.chamar<Resposta<"consultarCob", 200>>("GET", `/cob/${encodeURIComponent(txid)}`),
    listar: (filtro: Consulta<"listarCobs">) => this.chamar<Resposta<"listarCobs", 200>>("GET", "/cob" + query(filtro)),
    revisar: (txid: string, corpo: Corpo<"revisarCob">) =>
      this.chamar<Resposta<"revisarCob", 200>>("PATCH", `/cob/${encodeURIComponent(txid)}`, corpo),
    /** Cancela uma cobrança que ainda não foi paga. */
    cancelar: (txid: string) =>
      this.chamar<Resposta<"revisarCob", 200>>("PATCH", `/cob/${encodeURIComponent(txid)}`, { status: "REMOVIDA_PELO_USUARIO_RECEBEDOR" }),
  };

  /** Cobrança com vencimento (cobv). */
  readonly cobv = {
    criar: (txid: string, corpo: Corpo<"criarCobv">) =>
      this.chamar<Resposta<"criarCobv", 201>>("PUT", `/cobv/${encodeURIComponent(txid)}`, corpo),
    consultar: (txid: string) => this.chamar<Resposta<"consultarCobv", 200>>("GET", `/cobv/${encodeURIComponent(txid)}`),
    listar: (filtro: Consulta<"listarCobvs">) => this.chamar<Resposta<"listarCobvs", 200>>("GET", "/cobv" + query(filtro)),
    revisar: (txid: string, corpo: Corpo<"revisarCobv">) =>
      this.chamar<Resposta<"revisarCobv", 200>>("PATCH", `/cobv/${encodeURIComponent(txid)}`, corpo),
  };

  /** Pix recebidos, como o Vintém os gravou (pelo webhook do banco ou pela conciliação), com as devoluções. */
  readonly pix = {
    consultar: (endToEndId: string) => this.chamar<Resposta<"consultarPix", 200>>("GET", `/pix/${encodeURIComponent(endToEndId)}`),
    listar: (filtro: Consulta<"listarPix">) => this.chamar<Resposta<"listarPix", 200>>("GET", "/pix" + query(filtro)),
  };

  /** Devolução de um Pix recebido. O id identifica a devolução: repetir o mesmo id não devolve duas vezes. */
  readonly devolucao = {
    solicitar: (endToEndId: string, id: string, corpo: Corpo<"solicitarDevolucao">) =>
      this.chamar<Resposta<"solicitarDevolucao", 201>>("PUT", `/pix/${encodeURIComponent(endToEndId)}/devolucao/${encodeURIComponent(id)}`, corpo),
    consultar: (endToEndId: string, id: string) =>
      this.chamar<Resposta<"consultarDevolucao", 200>>("GET", `/pix/${encodeURIComponent(endToEndId)}/devolucao/${encodeURIComponent(id)}`),
  };

  private async chamar<T>(metodo: string, caminho: string, corpo?: unknown, headers: Record<string, string> = {}): Promise<T> {
    const resp = await this.fetch(this.url + caminho, {
      method: metodo,
      headers: {
        Authorization: `Bearer ${this.token}`,
        ...(corpo === undefined ? {} : { "Content-Type": "application/json" }),
        ...headers,
      },
      body: corpo === undefined ? undefined : JSON.stringify(corpo),
    });
    const texto = await resp.text();
    if (!resp.ok) {
      let problema: Problema | undefined;
      try {
        problema = JSON.parse(texto) as Problema;
      } catch {
        // corpo fora do formato: fica só o texto
      }
      throw new VintemError(resp.status, problema, texto);
    }
    return (texto ? JSON.parse(texto) : undefined) as T;
  }
}

/** Evento que o Vintém entrega no seu endpoint (Standard Webhooks). */
export interface EventoVintem<D = Record<string, unknown>> {
  type: string;
  timestamp: string;
  data: D;
}

/** Erro de verificação de webhook: assinatura errada, cabeçalho faltando ou fora da janela de tempo. */
export class AssinaturaInvalida extends Error {
  constructor(motivo: string) {
    super(motivo);
    this.name = "AssinaturaInvalida";
  }
}

/**
 * Confere a assinatura de um webhook do Vintém (Standard Webhooks) e devolve o
 * evento. Use o corpo cru, exatamente como chegou: reserializar o JSON muda a
 * assinatura. `segredo` é o whsec_ mostrado ao criar o destino.
 */
export async function verificarWebhook<D = Record<string, unknown>>(
  segredo: string,
  headers: Headers | Record<string, string | string[] | undefined>,
  corpo: string,
  opcoes: { toleranciaSegundos?: number; agora?: Date } = {},
): Promise<EventoVintem<D>> {
  const ler = (nome: string): string | undefined => {
    if (headers instanceof Headers) return headers.get(nome) ?? undefined;
    const v = headers[nome] ?? headers[nome.toLowerCase()];
    return Array.isArray(v) ? v[0] : v;
  };
  const id = ler("webhook-id");
  const ts = ler("webhook-timestamp");
  const assinaturas = ler("webhook-signature");
  if (!id || !ts || !assinaturas) throw new AssinaturaInvalida("faltam os cabeçalhos webhook-id, webhook-timestamp e webhook-signature");

  const segundos = Number(ts);
  const agora = Math.floor((opcoes.agora ?? new Date()).getTime() / 1000);
  if (!Number.isInteger(segundos) || Math.abs(agora - segundos) > (opcoes.toleranciaSegundos ?? 300)) {
    throw new AssinaturaInvalida("webhook-timestamp fora da janela de tolerância");
  }

  const chave = await crypto.subtle.importKey("raw", base64ParaBytes(segredo.replace(/^whsec_/, "")), { name: "HMAC", hash: "SHA-256" }, false, ["sign"]);
  const esperada = new Uint8Array(await crypto.subtle.sign("HMAC", chave, new TextEncoder().encode(`${id}.${ts}.${corpo}`)));
  const valida = assinaturas.split(" ").some((a) => {
    const [versao, valor] = a.split(",", 2);
    return versao === "v1" && valor !== undefined && iguaisEmTempoConstante(base64ParaBytes(valor), esperada);
  });
  if (!valida) throw new AssinaturaInvalida("assinatura não confere");
  return JSON.parse(corpo) as EventoVintem<D>;
}

function base64ParaBytes(b64: string): Uint8Array<ArrayBuffer> {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function iguaisEmTempoConstante(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false;
  let dif = 0;
  for (let i = 0; i < a.length; i++) dif |= a[i]! ^ b[i]!;
  return dif === 0;
}

function query(filtro: object): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(filtro)) if (v !== undefined && v !== null) q.set(k, String(v));
  const s = q.toString();
  return s ? `?${s}` : "";
}

function novoId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}
