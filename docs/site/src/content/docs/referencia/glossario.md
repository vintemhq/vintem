---
title: Glossário
description: Os termos do Pix e do Vintém, em uma linha cada.
---

| Termo | O que é |
|---|---|
| **Adapter** | A parte do Vintém que fala com um banco específico. Cada banco tem o seu |
| **API Pix** | A especificação do Banco Central que os bancos seguem. A API do Vintém segue o mesmo formato |
| **BCB** | Banco Central do Brasil, que define e opera o Pix |
| **BR Code** | O texto do Pix copia e cola, que também vira o QR Code. Fica em `pixCopiaECola` |
| **Chave Pix** | CPF, CNPJ, e-mail, celular ou chave aleatória que identifica a conta que recebe |
| **Chave mestra** | A `VINTEM_MASTER_KEY`, que cifra as credenciais dos bancos. Veja [segurança](/operar/seguranca/) |
| **cob** | [Cobrança imediata](/conceitos/cobrancas/), para pagar agora |
| **cobr** | [Cobrança recorrente](/conceitos/pix-automatico/): a cobrança de um ciclo do Pix Automático |
| **cobv** | [Cobrança com vencimento](/conceitos/cobrancas/), com multa, juros e desconto |
| **Conciliação** | [Consulta periódica ao banco](/conceitos/recebimentos-e-conciliacao/) que recupera o que não foi avisado |
| **Conta recebedora** | [Banco, chave Pix, credenciais e ambiente](/conceitos/contas-e-certificados/) cadastrados no Vintém |
| **Destino** | A URL da sua aplicação que recebe os [eventos](/conceitos/webhooks-de-saida/) |
| **Devolução** | [Mandar de volta ao pagador](/conceitos/devolucoes/) todo ou parte de um Pix recebido |
| **DLQ** | Fila das entregas de evento que falharam por 24 h, de onde podem ser reenviadas |
| **e2eid** | O `endToEndId`: o identificador único de um Pix, criado na transação |
| **Escopo** | [O que uma API key pode fazer](/conceitos/api-keys-e-escopos/), como `cob:write` |
| **Homologação** | O ambiente de testes do banco, sem dinheiro de verdade |
| **idRec** | O identificador de uma recorrência do Pix Automático |
| **Idempotência** | [Repetir uma requisição sem criar nada em dobro](/conceitos/idempotencia/), com o header `Idempotency-Key` |
| **ISPB** | O código de 8 caracteres que identifica uma instituição no sistema de pagamentos |
| **Jornada** | Cada uma das [quatro formas](/conceitos/pix-automatico/) de o pagador autorizar o Pix Automático |
| **Location** | O endereço no banco de onde o app do pagador busca os dados de uma cobrança dinâmica |
| **mTLS** | TLS com certificado dos dois lados. É como a sua conta se identifica para o banco |
| **Não custodial** | Que não guarda nem movimenta o dinheiro de ninguém, como o Vintém |
| **OAuth2** | O protocolo com que o Vintém obtém o token de acesso à API do banco |
| **Pix Automático** | [Pagamento recorrente](/conceitos/pix-automatico/): o pagador autoriza uma vez, e as cobranças seguintes são pagas sozinhas |
| **PSP** | Prestador de serviço de pagamento. Na prática, o banco da conta |
| **rec** | [Recorrência](/conceitos/pix-automatico/): o acordo do Pix Automático |
| **Revisão** | O campo `revisao`, que conta quantas vezes uma cobrança foi alterada. Começa em 0 |
| **rtrId** | O identificador da devolução no sistema do Pix |
| **Sandbox** | O [banco de mentira do Vintém](/gerado/sandbox/), com estado, para testar sem conta |
| **solicrec** | [Solicitação](/conceitos/pix-automatico/): o pedido de autorização enviado ao app do pagador |
| **Standard Webhooks** | O [padrão aberto](https://www.standardwebhooks.com/) de assinatura que os eventos do Vintém seguem |
| **Tarifa** | O que o banco cobra por Pix recebido. O Vintém não soma nada a ela |
| **txid** | O identificador da cobrança, com 26 a 35 caracteres. Ele liga o Pix à cobrança |
| **Webhook** | Um aviso por HTTP. O [de entrada](/conceitos/recebimentos-e-conciliacao/) vem do banco, e o [de saída](/conceitos/webhooks-de-saida/) vai para a sua aplicação |
