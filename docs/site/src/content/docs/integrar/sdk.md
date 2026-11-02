---
title: SDK TypeScript
description: "@vintemhq/sdk: cliente tipado do Vintém, gerado do OpenAPI, com verificação de webhook."
---

`@vintemhq/sdk` é gerado do `openapi.yaml`, não tem dependências e roda em Node
20+, Deno, Bun e navegadores.

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

`vintem.cob`, `vintem.cobv` e `vintem.devolucao` cobrem criar, consultar,
listar, revisar e cancelar, e `vintem.pix` lista e consulta os Pix recebidos. Erro da API vira `VintemError`, com `status` e o
`problema` no formato da spec do BCB.

## Verificar um webhook

O Vintém assina os eventos no padrão [Standard Webhooks](https://www.standardwebhooks.com/).
Confira com o corpo **cru**, do jeito que chegou:

```ts
import { verificarWebhook } from "@vintemhq/sdk";

const evento = await verificarWebhook(process.env.VINTEM_WEBHOOK_SECRET!, req.headers, corpoCru);
```

Assinatura errada, corpo alterado ou horário fora da janela de 5 minutos lançam
`AssinaturaInvalida`. O código está em
[`sdk/typescript`](https://github.com/vintemhq/vintem/tree/develop/sdk/typescript).
