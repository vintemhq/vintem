<div align="center">

# Vintém

### Pix without a gateway. Your bank account, your infrastructure, no middleman fee.

**Self-hosted, multi-bank Pix gateway with Pix Automático (recurring Pix).**
One binary that speaks the Central Bank's Pix API, ships an adapter per bank,
handles the mTLS certificates and normalizes webhooks.

<sub>*Vintém* was an old Brazilian 20-réis coin, worth almost nothing. Not losing a single one to a middleman is the point.</sub>

<img src="docs/demo/sandbox.gif" alt="vintem sandbox: a Pix charge is created and paid in a few seconds" width="820">

[Português](./README.pt-BR.md) · [Docs](https://vintem.dev/) · [Roadmap](https://github.com/orgs/vintemhq/projects/1)

</div>

> 🚧 **Alpha.** The sandbox works, and the Efí adapter has received real Pix payments. The other banks are not yet tested against the real bank, and the API may still change. Watch the repo or follow the board.

## Try it in 10 seconds

```bash
docker run --rm -p 8080:8080 ghcr.io/vintemhq/vintem sandbox
```

It prints a test API key and a `curl` that creates a Pix charge. Paste it: five
seconds later the fake payer pays and the charge turns `CONCLUIDA`. No bank
account, no config. Without Docker: `go run github.com/vintemhq/vintem/cmd/vintem@latest sandbox`.
See [the sandbox guide (pt-BR)](./docs/SANDBOX.md) for magic values.

## Use it from your agent

`vintem mcp` is an MCP server for AI agents: create, check, list and cancel Pix
charges, issue charges with a due date, and refund payments. It talks to the Vintém API with an API key, so the
key's scopes apply — a read-only key gives you an agent that can only look.

```bash
vintem sandbox   # copy the vtm_... key it prints
claude mcp add vintem -e VINTEM_TOKEN=vtm_... -e VINTEM_CHAVE_PIX=sandbox@vintem.dev -- vintem mcp
```

Then ask: *"charge R$ 49.90 for order 42 and show me the Pix code"*. Any MCP
client works the same way: run `vintem mcp` over stdio with `VINTEM_URL`
(default `http://localhost:8080`), `VINTEM_TOKEN` and, optionally,
`VINTEM_CHAVE_PIX`.

## Use it from TypeScript

```ts
import { Vintem } from "@vintemhq/sdk";

const vintem = new Vintem({ url: "http://localhost:8080", token: process.env.VINTEM_TOKEN! });
const cob = await vintem.cob.criar({ calendario: { expiracao: 3600 }, valor: { original: "49.90" }, chave: "sandbox@vintem.dev" });
console.log(cob.pixCopiaECola);
```

Typed from the OpenAPI spec, zero dependencies, with webhook signature
verification. See [`sdk/typescript`](./sdk/typescript).

## Why

Payment gateways charge ~1% per Pix, on top of being one more company between
you and your money. Every bank already exposes the official
[BCB Pix API](https://github.com/bacen/pix-api), but each one deviates from it,
requires its own mTLS certificate dance, and sends webhooks in its own shape.

Vintém is the missing layer: you run it, you plug in your own bank credentials,
and your app talks to **one** API no matter which bank receives the money.

## What you pay

Vintém is free and adds nothing per Pix. You pay **your bank's** Pix fee, and
that is the whole bill. How much you save depends on the bank:

| Where the Pix goes through | Fee per Pix | R$ 50 sale | R$ 500 sale |
|---|---|---|---|
| [Stripe](https://stripe.com/br/pricing) (Pix, invite only) | 1.19% | R$ 0.60 | R$ 5.95 |
| [Mercado Pago](https://www.mercadopago.com.br/ferramentas-para-vender/check-out) (checkout, paid instantly) | 0.99% | R$ 0.50 | R$ 4.95 |
| [Asaas](https://www.asaas.com/precos-e-taxas) | R$ 2.15 flat (R$ 1.08 for the first 3 months) | R$ 2.15 | R$ 2.15 |
| **Vintém + [Efí](https://sejaefi.com.br/tarifas)** | 1.19% (Efí's fee; Vintém adds 0) | R$ 0.60 | R$ 5.95 |

<sub>Public price pages, checked on 29/10/2026. Your negotiated rate may differ.</sub>

**Be honest with yourself:** with Efí, the fee is the same as Stripe's. What
you gain today is ownership — your data, your webhooks, no lock-in, and moving
to another bank without rewriting your integration. The fee drops with banks
that cap or flatten the Pix fee for business accounts. The Inter adapter is
ready to try (Inter has publicly stated 0.9% capped at R$ 1.50 for dynamic QR
codes, but we could not confirm it on a current official price page). So are
Sicoob, where the Pix fee is whatever your credit union negotiates, and Banco
do Brasil.

## What it does

- **One API, many banks** — Efí, Inter, Sicoob and Banco do Brasil today. Every adapter passes the same [conformance suite](./CONTRIBUTING.md#suíte-de-conformidade).
- **Charges** — immediate (`cob`) and with due date (`cobv`), BR Code / QR generation built in.
- **Pix Automático** — `rec`, `solicrec`, `cobr` and recurring webhooks.
- **Webhooks that don't get lost** — normalized, signed ([Standard Webhooks](https://www.standardwebhooks.com/)), retried, plus a reconciliation job that recovers missed payments.
- **Refunds** — total or partial, never more than was received.
- **Stateful sandbox** — test the full flow without a business bank account.
- **Single binary** — `docker run` and go; SQLite by default.
- **Dashboard** — `/painel/`: money received, active charges, what a gateway would have charged, and each charge with its QR code. Log in with an API key.
- **Use it from your agent** — built-in MCP server (`vintem mcp`).

## Banks

| Bank | `--psp` | cob | cobv | Refund | Webhook | Status |
|---|---|---|---|---|---|---|
| Vintém sandbox | `sandbox` | ✅ | ✅ | ✅ | ✅ | For testing without a business account ([guide, pt-BR](./docs/SANDBOX.md)) |
| Efí | `efi` | ✅ | ✅ | ✅ | ✅ | **Tested in production** with real money: charge, webhook, reconciliation and refund; `cobv` only against homologation ([guide, pt-BR](./docs/EFI.md)) |
| Banco Inter | `inter` | ✅ | ✅ | ✅ | ✅ | Written from Inter's official API spec; **not yet tested against Inter's sandbox** ([guide, pt-BR](./docs/INTER.md)) |
| Sicoob | `sicoob` | ✅ | ✅ | ✅ | ✅ | Written from the BCB spec and public sandbox reports; **not yet tested with a member account** ([guide, pt-BR](./docs/SICOOB.md)) |
| Banco do Brasil | `bb` | ✅ | ✅ | ✅ | ✅ | Written from the BCB spec and public homologation reports; **not yet tested by Vintém against BB's homologation** ([guide, pt-BR](./docs/BB.md)) |

## Non-custodial

Vintém never holds or moves money. It runs with **your** credentials against
**your** bank account. This is not legal or regulatory advice.

## License

[Apache-2.0](./LICENSE). Pix is a trademark of Banco Central do Brasil;
Vintém is an independent project, not affiliated with or endorsed by the BCB.
