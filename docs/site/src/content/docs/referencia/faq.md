---
title: Perguntas frequentes
description: Respostas diretas sobre custo, segurança, bancos testados e o que o Vintém ainda não faz.
---

## É grátis?

O software é, sob a [licença Apache 2.0](https://github.com/vintemhq/vintem/blob/develop/LICENSE), e não cobra nada por Pix. Você paga a tarifa de Pix do seu banco e a infraestrutura onde ele roda. Veja a [comparação de taxas](/#quanto-você-paga).

Não há versão hospedada: você mesmo roda o Vintém.

## O dinheiro passa pelo Vintém?

Não. O Pix cai direto na sua conta. O Vintém usa as suas credenciais para falar com o seu banco e nunca guarda nem movimenta dinheiro ([ADR-004](/gerado/adr/004-nao-custodial/)).

## Quais bancos estão testados?

Só a **Efí** recebeu Pix de verdade: cobrança imediata, webhook, conciliação e devolução, numa conta real. A cobrança com vencimento dela só passou pela homologação.

**Inter, Sicoob e Banco do Brasil não foram testados contra o banco real.** Os adapters seguem a documentação e passam na suíte de conformidade contra um banco simulado. Nosso primeiro teste real na Efí achou dois defeitos que a simulação não pegou, então espere encontrar algo parecido nesses. Comece com centavos e [conte o que encontrou](https://github.com/vintemhq/vintem/issues).

## Dá para usar em produção?

Com a Efí, dá, sabendo que o projeto é **alpha**: ele é novo e a API pode mudar antes da `v1.0.0`, com a quebra anotada no release. Siga o [guia de produção](/operar/producao/), teste a restauração do backup e comece com valores baixos.

## Preciso de CNPJ?

Depende do que você quer usar:

| Para | Precisa de |
|---|---|
| Testar | Nada: o [sandbox](/gerado/sandbox/) não usa banco |
| Receber pela Efí | Conta Efí com o menu API. Testamos com uma conta de pessoa física (CPF) |
| Inter, Sicoob ou Banco do Brasil | Conta PJ |
| Pix Automático em banco real | Conta PJ, e ainda não está disponível |

## O Pix Automático funciona?

Só no sandbox. A API está pronta, mas nenhum adapter de banco real a implementa ainda, então pedir isso numa conta real responde [`NaoSuportado`](/erros/naosuportado/). Veja [como funciona](/conceitos/pix-automatico/).

## Posso devolver o valor integral de um Pix?

Só se a conta tiver saldo para isso. O banco desconta a tarifa ao receber: na Efí, um Pix de R$ 10,00 deixou R$ 9,88 na conta. Sem outro saldo, uma devolução de R$ 10,00 é aceita e depois termina `NAO_REALIZADO`. Veja [devoluções](/conceitos/devolucoes/).

## Onde ficam os dados?

Num arquivo SQLite, na sua máquina. As credenciais dos bancos ficam cifradas com a sua chave mestra. O Vintém não envia dados para o projeto. Veja [segurança](/operar/seguranca/).

## E se o Vintém ficar fora do ar?

O Pix continua caindo na sua conta, porque ele não passa pelo Vintém. O que para é a criação de cobranças. Quando ele voltar, a [conciliação](/conceitos/recebimentos-e-conciliacao/) encontra o que aconteceu no intervalo, dentro da janela configurada.

## Consigo trocar de banco depois?

Sua aplicação continua falando a mesma API: cadastre uma conta no banco novo e use a chave Pix dela nas cobranças novas. As cobranças antigas continuam na conta antiga.

## O que ele não faz?

Boleto, cartão, split de pagamento e emissão de nota fiscal estão fora do escopo. O Vintém também não envia Pix: ele só recebe e devolve.

## Como eu contribuo com um banco?

O [guia de contribuição](/gerado/contribuir/) mostra o caminho. Banco que segue a API Pix do BCB aproveita quase tudo o que já existe, e há [issues abertas para outros bancos](https://github.com/vintemhq/vintem/issues?q=is%3Aissue+is%3Aopen+label%3Aarea%3Aadapters).

## Por que esse nome?

O vintém era uma moeda de pouco valor, e não perder um vintém para intermediário é a promessa do projeto. Ele se chamava Lastro até a `v0.8.0` e mudou por um conflito de marca ([ADR-009](/gerado/adr/009-nome-e-marca/)).

## O Vintém tem relação com o Banco Central?

Não. Pix é marca do Banco Central do Brasil, e o Vintém é um projeto independente, sem vínculo nem endosso do BCB. Nada aqui é aconselhamento jurídico ou regulatório.
