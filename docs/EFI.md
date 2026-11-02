<div align="center">

# 🏦 Vintém — Efí

### Receber Pix pela sua conta Efí, com o Vintém no meio.

</div>

> ✅ **Testado em produção em 30/10/2026**, com uma conta Efí Pro (CPF): cobrança real
> paga, webhook do banco, conciliação e devolução. Cobrança com vencimento só foi testada
> na homologação. A suíte de conformidade roda sobre fitas da homologação. Acompanhe pela
> [#27](https://github.com/vintemhq/vintem/issues/27).

## 1. Conta e credenciais

1. **Conta Digital Efí com o menu "API".** É o que a [documentação da
   Efí](https://dev.efipay.com.br/docs/api-pix/credenciais) pede; conta de
   pessoa física também serve se o menu aparecer. O menu só existe no painel
   web, não no app.
2. No painel, em **API → Aplicações**, crie uma aplicação e ative a **API Pix**
   com os escopos de **homologação** (e de produção, quando for a hora):
   os escopos que o Vintém usa são `cob.write`, `cob.read`, `cobv.write`,
   `cobv.read`, `pix.read` (Pix recebidos), `pix.write` (devolução) e
   `webhook.write`.
   No painel eles aparecem como Alterar/Consultar cobranças, Alterar/Consultar
   Pix, Alterar Webhooks e Alterar/Consultar cobranças com vencimento. Não
   marque Enviar Pix nem Pagar QR Code: o Vintém só recebe. O painel não deixa
   criar a aplicação sem ao menos um escopo de **produção**; para testar só na
   homologação, marque em produção apenas "Consultar cobranças", que só lê.
3. Copie o **Client ID** e o **Client Secret** do ambiente que vai usar. Cada
   aplicação tem um par para produção e outro para homologação.
4. Em **API → Meus Certificados**, gere o certificado do **mesmo ambiente** e
   baixe o `.p12`. **O download só acontece na criação.**

## 2. Cadastrar a conta no Vintém

Segredos vêm de arquivo, nunca de flag, porque flag fica no histórico do shell:

```sh
printf '%s' 'SEU_CLIENT_SECRET' > secret.txt   # ou crie o arquivo num editor
vintem conta criar --id minha-loja --nome "Minha Loja" --psp efi \
  --chave sua-chave-pix@exemplo.com --ambiente homologacao \
  --client-id SEU_CLIENT_ID --client-secret-arquivo secret.txt \
  --certificado homologacao.p12
```

Se o `.p12` tiver senha, passe também `--senha-certificado-arquivo`. O certificado é conferido na hora (RN-62), e o
`conta listar` mostra até quando ele vale. Depois, apague `secret.txt`: o
Vintém guarda tudo cifrado com a `VINTEM_MASTER_KEY`.

**A chave Pix precisa ser da conta Efí**; senão o Efí recusa a cobrança com
`chave_invalida`.

## 3. Webhook

```sh
vintem conta webhook minha-loja --url-publica https://seu-vintem.exemplo.com
```

O Vintém gera uma URL com um token secreto e a registra na sua chave Pix
(`PUT /v2/webhook/:chave`). O Efí acrescenta `/pix` ao fim, e a rota do Vintém
aceita os dois caminhos. Ao registrar, o Efí manda uma notificação de teste, que o
Vintém aceita sem registrar nada.

**Sobre o mTLS do webhook.** O Efí exige mTLS no servidor que recebe o aviso. O
Vintém não termina TLS, então registra o webhook com `x-skip-mtls-checking: true`.
Nesse modo, o Efí pede que você garanta de outro jeito que o aviso veio dele:

- **O token secreto na URL** já faz o papel do "hmac na URL" que o Efí sugere.
  Aviso sem o token certo responde 401.
- **Restrinja a origem no seu proxy** ao IP que o Efí publica (hoje
  `34.193.116.226`; confira na [documentação de webhooks](https://dev.efipay.com.br/docs/api-pix/webhooks/)).

Se o webhook falhar ou sumir, a **conciliação** busca os Pix em `GET /v2/pix` a
cada 5 minutos (RN-31). Nenhum pagamento depende só do aviso.

**Para cancelar,** por exemplo depois de um teste com túnel temporário:

```sh
vintem conta webhook minha-loja --remover
```

O Vintém chama `DELETE /v2/webhook/:chave` e invalida a URL. Antes dele, o webhook
só saía pela API do Efí: quem já tinha excluído a aplicação precisava criar outra
para isso. Faça antes de excluir a aplicação no Efí.

## 4. Testar na homologação

- **Só cobranças de R$ 0,01 a R$ 10,00 são confirmadas e geram webhook.** Acima
  disso, a cobrança fica ATIVA para sempre.
- **Devolução de R$ 0,01 é rejeitada** de propósito (simulação de falha).

## O que observamos em produção

- **A tarifa sai do valor recebido, e isso limita a devolução.** Numa conta Efí Pro (CPF),
  um Pix de R$ 1,00 custou R$ 0,01, e um de R$ 10,00 custou R$ 0,12, o que bate com a
  [tarifa pública de 1,19%](https://sejaefi.com.br/tarifas). A devolução não teve tarifa. Sem outro saldo na conta, não dá para devolver o valor
  integral: a devolução de R$ 1,00 foi aceita e depois terminou `NAO_REALIZADO`, sem motivo.
  O valor não realizado volta a ficar disponível. Acompanhe o resultado pelo evento
  `devolucao.atualizada`, não pela resposta do `PUT`. Confira a tarifa da sua conta no painel.
- **O registro do webhook chama a URL.** Ela precisa estar no ar e responder na hora do
  `vintem conta webhook`, ou o Efí recusa com `webhook_invalido`.
- **A descrição da devolução não vai ao Efí,** que recusa o campo. Ela fica só no Vintém.

## Desvios do Efí tratados pelo adapter

Estão no código, em [`internal/adapters/psp/efi/doc.go`](../internal/adapters/psp/efi/doc.go):

- **Erros:** o Efí responde `{"nome","mensagem"}` na cob e o formato de problema na cobv.
- **`expiracao`** vem como string no `GET`.
- **`GET /v2/pix`** filtra com `fim` inclusivo.
- **Devolução com id repetido** responde 409; o adapter a torna idempotente.
- ⚠️ **A confirmar na homologação:** o erro de `txid` repetido na cobv, que a
  documentação não nomeia.
