---
title: Use com seu agente (MCP)
description: vintem mcp dá a agentes de IA ferramentas para cobrar, consultar e devolver Pix, com os escopos da API key.
---

`vintem mcp` é um servidor MCP por stdio. Ele chama a API do Vintém com a API
key do agente, então os escopos valem: uma key só de leitura dá um agente que
só consulta ([ADR-008](/gerado/adr/008-mcp-como-cliente-da-api/)).

## No Claude Code

```bash
vintem sandbox   # copie a key vtm_... que ele imprime
claude mcp add vintem -e VINTEM_TOKEN=vtm_... -e VINTEM_CHAVE_PIX=sandbox@vintem.dev -- vintem mcp
```

Depois peça: *"cobre R$ 49,90 do pedido 42 e me mostre o Pix copia e cola"*.

## Em qualquer cliente MCP

Rode `vintem mcp` por stdio com estas variáveis:

| Variável | Para quê |
|---|---|
| `VINTEM_URL` | Onde está a API (padrão `http://localhost:8080`) |
| `VINTEM_TOKEN` | A API key do agente |
| `VINTEM_CHAVE_PIX` | Chave usada quando o agente não informa outra |

## Ferramentas

| Ferramenta | Escopo |
|---|---|
| `criar_cobranca` | `cob:write` |
| `consultar_cobranca` | `cob:read` |
| `listar_cobrancas` | `cob:read` |
| `cancelar_cobranca` | `cob:write` |
| `devolver_pix` | `devolucao:write` |
| `consultar_devolucao` | `pix:read` |

Sem `inicio` e `fim`, `listar_cobrancas` lista as cobranças das últimas 24 horas. Para
outro período, informe os dois em RFC 3339.
