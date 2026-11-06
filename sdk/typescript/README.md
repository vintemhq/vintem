# @vintemhq/sdk

SDK TypeScript do [Vintém](https://github.com/vintemhq/vintem), o gateway Pix
self-hosted. Os tipos são gerados do `api/openapi.yaml`, então acompanham a API.
Sem dependências; roda em Node 20+, Deno, Bun e navegadores.

```bash
npm install @vintemhq/sdk
```

## Criar uma cobrança

```ts
import { Vintem } from "@vintemhq/sdk";

const vintem = new Vintem({ url: "http://localhost:8080", token: process.env.VINTEM_TOKEN! });
const cob = await vintem.cob.criar({
  calendario: { expiracao: 3600 },
  valor: { original: "49.90" },
  chave: "sandbox@vintem.dev",
});
console.log(cob.txid, cob.status, cob.pixCopiaECola);
```

Para testar sem banco, suba o `vintem sandbox` e use a API key que ele imprime.

| | |
|---|---|
| `vintem.cob` | `criar`, `criarComTxid`, `consultar`, `listar`, `revisar`, `cancelar` |
| `vintem.cobv` | `criar`, `consultar`, `listar`, `revisar` |
| `vintem.pix` | `listar`, `consultar` |
| `vintem.devolucao` | `solicitar`, `consultar` |

Erro da API vira `VintemError`, com `status` e o `problema` no formato da spec
do BCB. `cob.criar` manda uma `Idempotency-Key` nova a cada chamada; passe a sua
(`{ idempotencyKey }`) para tentar de novo sem criar duas cobranças.

## Verificar um webhook

O Vintém assina os eventos no padrão [Standard Webhooks](https://www.standardwebhooks.com/).
Confira com o corpo **cru**, como chegou:

```ts
import { verificarWebhook, AssinaturaInvalida } from "@vintemhq/sdk";

app.post("/webhooks/vintem", express.text({ type: "*/*" }), async (req, res) => {
  try {
    const evento = await verificarWebhook(process.env.VINTEM_WEBHOOK_SECRET!, req.headers, req.body);
    // evento.type, evento.data
    res.sendStatus(204);
  } catch (e) {
    res.sendStatus(e instanceof AssinaturaInvalida ? 401 : 500);
  }
});
```

## Desenvolvimento

```bash
npm ci
npm run gerar   # regenera src/schema.ts do api/openapi.yaml
VINTEM_URL=http://localhost:8080 VINTEM_TOKEN=vtm_... npm test
```

Sem `VINTEM_URL`/`VINTEM_TOKEN`, os testes contra o sandbox são pulados. A
publicação no npm sai do CI a partir de uma tag `sdk-vX.Y.Z` igual à versão do
`package.json`.
