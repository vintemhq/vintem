---
title: CLI
description: Todos os comandos e flags do vintem.
---

Sem comando, `vintem` roda `serve`. Em qualquer comando, `-h` mostra a ajuda, sem
precisar da chave mestra.

Em Docker, o binário fica em `/vintem`:

```bash
docker compose exec vintem /vintem conta listar
```

Os comandos que abrem o banco precisam de `VINTEM_MASTER_KEY` e leem `VINTEM_DB`. Veja
a [configuração](/operar/configuracao/).

## Servidor

| Comando | O que faz |
|---|---|
| `vintem serve` | Sobe a API, o painel e os jobs |
| `vintem sandbox` | Sobe tudo pronto para testar, sem banco nem configuração. Veja o [sandbox](/gerado/sandbox/) |
| `vintem mcp` | Servidor [MCP](/integrar/mcp/) para agentes, por stdio |
| `vintem version` | Mostra a versão |
| `vintem chave gerar` | Gera uma `VINTEM_MASTER_KEY` nova |
| `vintem chave trocar` | Recifra as credenciais e os segredos com `VINTEM_MASTER_KEY_NOVA`. Veja [como trocar](/operar/seguranca/#trocar-a-chave-mestra) |

## Contas

| Comando | O que faz |
|---|---|
| `vintem conta criar` | Cadastra uma conta recebedora |
| `vintem conta listar` | Lista as contas, com a validade do certificado |
| `vintem conta desativar <id>` | Impede novas cobranças na conta |
| `vintem conta webhook <id>` | Gera a URL de aviso de Pix e a registra no banco. Rodar de novo troca o token. Com `--remover`, cancela |

Flags de `vintem conta criar`:

| Flag | O que é |
|---|---|
| `--id` | Identificador da conta (`[a-z0-9-]`) |
| `--nome` | Nome para exibição |
| `--psp` | Adapter do banco: `sandbox`, `efi`, `inter`, `sicoob` ou `bb` |
| `--chave` | Chave Pix que recebe as cobranças |
| `--ambiente` | `homologacao` (padrão) ou `producao` |
| `--client-id` | Client id do OAuth2 do banco |
| `--client-secret-arquivo` | Arquivo com o client secret |
| `--certificado` | Arquivo do certificado do mTLS (`.p12` ou `.pem`) |
| `--senha-certificado-arquivo` | Arquivo com a senha do certificado |
| `--chave-aplicacao` | Chave de aplicação do portal do BB (`gw-dev-app-key`); só para o `bb` |

Flags de `vintem conta webhook`:

| Flag | O que é |
|---|---|
| `--url-publica` | Endereço público deste Vintém, como `https://vintem.suaempresa.com` |
| `--remover` | Cancela o aviso registrado no banco e invalida a URL. Use depois de um teste, para a chave Pix não ficar apontando para uma URL morta. Se o banco falhar, a URL continua valendo |

## API keys

| Comando | O que faz |
|---|---|
| `vintem apikey criar` | Cria uma API key. O token aparece uma vez só |
| `vintem apikey listar` | Lista as API keys |
| `vintem apikey revogar <id>` | Revoga uma API key na hora |

Flags de `vintem apikey criar`:

| Flag | O que é |
|---|---|
| `--nome` | Para que serve a key, como `loja-producao` |
| `--escopo` | [Escopo](/conceitos/api-keys-e-escopos/) concedido; repita para dar mais de um |
| `--conta` | Restringe a key a uma conta |

## Eventos

| Comando | O que faz |
|---|---|
| `vintem destino criar` | Cadastra onde entregar os [eventos](/conceitos/webhooks-de-saida/). O segredo aparece uma vez só |
| `vintem destino listar` | Lista os destinos |
| `vintem destino remover <id>` | Remove um destino. As entregas pendentes dele vão para a DLQ |
| `vintem dlq listar` | Lista as entregas que desistiram depois de 24 h |
| `vintem dlq reenviar <evento-id>` | Devolve as entregas do evento à fila |

Flags de `vintem destino criar`:

| Flag | O que é |
|---|---|
| `--url` | URL da sua aplicação que recebe os eventos |
| `--conta` | Só os eventos desta conta. Sem ela, todas |

## Conciliação

| Comando | O que faz |
|---|---|
| `vintem conciliar` | Roda uma rodada da [conciliação](/conceitos/recebimentos-e-conciliacao/) na hora |
