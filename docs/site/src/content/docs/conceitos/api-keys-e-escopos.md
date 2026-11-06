---
title: API keys e escopos
description: Como autenticar na API, o que cada escopo libera e como restringir uma key a uma conta.
---

Toda rota, exceto `/health` e os webhooks de entrada, exige uma API key:

```http
Authorization: Bearer vtm_<id>_<segredo>
```

## Criar

```bash
vintem apikey criar --nome minha-app --escopo cob:read --escopo cob:write
```

O token aparece **uma vez**, na criação. O Vintém guarda só o hash. As keys são
criadas e revogadas pela CLI, que é também como se cria a primeira key de uma instância
nova.

```bash
vintem apikey listar
vintem apikey revogar <id>
```

A revogação vale na hora. Para rotacionar, crie a key nova, troque na aplicação e
revogue a antiga.

## Escopos

Cada rota exige exatamente um escopo. Dê a cada aplicação só o que ela precisa.

| Escopo | O que libera |
|---|---|
| `cob:read` | `GET /cob/{txid}` e `GET /cob` |
| `cob:write` | `POST /cob`, `PUT /cob/{txid}` e `PATCH /cob/{txid}` |
| `cobv:read` | `GET /cobv/{txid}` e `GET /cobv` |
| `cobv:write` | `PUT /cobv/{txid}` e `PATCH /cobv/{txid}` |
| `pix:read` | `GET /pix`, `GET /pix/{e2eid}` e `GET /pix/{e2eid}/devolucao/{id}` |
| `devolucao:write` | `PUT /pix/{e2eid}/devolucao/{id}` |
| `rec:read` | `GET /rec/{idRec}` e `GET /solicrec/{idSolicRec}` |
| `rec:write` | `POST /rec`, `POST /solicrec` e `PATCH /solicrec/{idSolicRec}` |
| `cobr:read` | `GET /cobr/{txid}` |
| `cobr:write` | `POST /cobr`, `PUT /cobr/{txid}`, `PATCH /cobr/{txid}` e `POST /cobr/{txid}/retentativa/{data}` |
| `webhook:admin` | `GET /vintem/v1/webhooks/entregas` e `POST /vintem/v1/webhooks/entregas/{eventoId}/reenviar` |
| `conta:admin` | `GET` e `POST /vintem/v1/contas`, `GET /vintem/v1/contas/{id}` e `POST /vintem/v1/contas/{id}/desativar` |

Sem o escopo, a resposta é **403** [`AcessoNegado`](/erros/acessonegado/), e o `detail`
diz qual faltou:

```json
{
  "type": "https://vintem.dev/erros/AcessoNegado",
  "title": "A API key não tem o escopo necessário",
  "status": 403,
  "detail": "api key sem o escopo necessário: precisa de cob:write"
}
```

Uma rota sem escopo mapeado é recusada, e um teste aponta a rota esquecida. Assim, uma
rota nova não nasce aberta por engano.

## Restringir a uma conta

```bash
vintem apikey criar --nome loja-a --conta loja-a --escopo cob:read --escopo cob:write
```

Uma key restrita só enxerga a conta dela:

- cobrança de outra conta responde **404**, sem revelar que existe;
- chave Pix de outra conta no corpo é tratada como chave desconhecida;
- ela não pode ter o escopo `conta:admin`.

A key é obrigatoriamente restrita ao criar uma [recorrência](/conceitos/pix-automatico/),
porque é ela que identifica o recebedor.

:::caution
A key impressa pelo `vintem sandbox` tem todos os escopos e não é restrita a uma conta.
Ela existe só para testar.
:::

## Falhas de autenticação

Header ausente, key malformada, revogada ou inexistente respondem o mesmo **401**
[`NaoAutenticado`](/erros/naoautenticado/), sem `detail`, para não revelar se a key
existe.

A regra completa é a [RN-60](/gerado/regras-de-negocio/). Para o painel, veja a
[key sugerida](/integrar/painel/).
