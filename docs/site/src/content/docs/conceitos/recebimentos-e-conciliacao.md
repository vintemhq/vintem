---
title: Recebimentos e conciliação
description: Como o Pix chega ao Vintém pelo webhook do banco e como a conciliação recupera o aviso que se perdeu.
---

Um Pix chega ao Vintém por dois caminhos: o aviso do banco e a consulta periódica.
Os dois gravam do mesmo jeito, então não importa qual chegou primeiro.

## O webhook do banco

```bash
vintem conta webhook minha-loja --url-publica https://vintem.suaempresa.com
```

O comando gera a URL de aviso da conta e a registra no banco:

```
https://vintem.suaempresa.com/webhooks/pix/{conta}/{token}
```

- **Trate a URL como segredo.** O token faz parte dela. Só o hash é gravado, e rodar
  o comando de novo troca o token.
- **A rota não usa API key,** porque quem chama é o banco. Token ausente ou errado
  responde **401** [`NaoAutenticado`](/erros/naoautenticado/), sem dizer o que houve.
- **Para cancelar,** rode `vintem conta webhook minha-loja --remover`. O aviso sai do
  banco e a URL deixa de valer. Faça isso depois de um teste, para a chave Pix não
  ficar apontando para uma URL morta. Se o banco falhar, nada muda aqui.
- **O banco acrescenta `/pix` à URL,** como manda a spec. Os dois caminhos são aceitos.
- **Um item inválido recusa o lote inteiro,** com **400**
  [`WebhookInvalido`](/erros/webhookinvalido/). É melhor o banco reenviar do que um
  Pix se perder em silêncio.

## O que acontece quando um Pix chega

Tudo numa transação só, e o evento é gravado antes de responder 200 ao banco:

1. O Pix é registrado. O `endToEndId` deduplica: o mesmo Pix avisado duas vezes gera
   um registro e um evento.
2. Se o `txid` é de uma `cob` ou `cobv` ativa da mesma conta, a cobrança é concluída.
3. Os eventos `pix.recebido` e `cobranca.concluida` entram na fila de saída.

Pix sem `txid`, ou com `txid` de outra conta, é registrado sem concluir nada. Se a
gravação falhar, o Vintém responde 5xx e o banco reenvia.

## A conciliação

Webhook falha: o banco pode não avisar, ou o seu servidor pode estar fora do ar. Por
isso, um job dentro do `vintem serve` consulta o `GET /pix` do banco de cada conta e
registra o que não chegou. **Nenhum Pix some** porque o banco falhou em avisar.

| Ajuste | Padrão | Variável |
|---|---|---|
| Frequência | A cada 5 minutos | `VINTEM_CONCILIACAO_INTERVALO` (`0` desliga, com aviso no log) |
| Janela consultada | As últimas 2 horas | `VINTEM_CONCILIACAO_JANELA` (zero é recusado) |

Depois de uma indisponibilidade, rode uma rodada na hora:

```bash
vintem conciliar
```

:::caution
A rodada automática só olha a janela configurada. Se o Vintém ficou fora do ar por
mais tempo que isso, aumente `VINTEM_CONCILIACAO_JANELA` antes de rodar a conciliação.
:::

Além dos Pix, a mesma rodada consulta no banco:

- as devoluções ainda `EM_PROCESSAMENTO`;
- as recorrências abertas (`CRIADA` e `APROVADA`) e as cobranças recorrentes abertas,
  nos bancos com Pix Automático.

### Como a rodada se comporta

- **Conta desativada também entra,** porque as cobranças antigas dela ainda podem ser pagas.
- **Uma conta que falha não impede as outras.** A falha vai para o log, e a rodada
  seguinte tenta de novo.
- **As rodadas nunca se sobrepõem,** e um atraso aleatório de até 10% evita que várias
  instâncias consultem o banco no mesmo segundo.

No sandbox, a conciliação roda a cada 10 segundos, e é ela que encontra os pagamentos.

## Consultar os Pix recebidos

Os Pix que o Vintém gravou saem pelas mesmas rotas da spec do BCB, com o escopo `pix:read`:

```bash
curl "http://localhost:8080/pix?inicio=2026-11-01T00:00:00Z&fim=2026-11-02T00:00:00Z" \
  -H "Authorization: Bearer vtm_..."
curl http://localhost:8080/pix/E00000000202611021130NCMDKVJFYNX -H "Authorization: Bearer vtm_..."
```

- **A janela é pelo horário do Pix,** e não pela criação da cobrança.
- **Filtros:** `txid`, `txIdPresente` (só Pix de cobrança, ou só os sem), `devolucaoPresente`,
  e `cpf` ou `cnpj` do pagador. A paginação é a mesma do `GET /cob`.
- **Cada Pix vem com as devoluções,** em `devolucoes`.
- **É o que o Vintém conhece,** não uma consulta ao banco: um Pix que ainda não foi
  avisado nem conciliado não aparece. Rode `vintem conciliar` se precisar dele agora.

As regras completas são a [RN-30 e a RN-31](/gerado/regras-de-negocio/).
