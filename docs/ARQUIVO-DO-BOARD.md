<div align="center">

# 🗄️ Vintém — Arquivo do board

### O que já foi entregue, com o caminho de volta ao código.

<sub>📋 board no <a href="https://github.com/orgs/vintemhq/projects">GitHub Projects</a> &nbsp;•&nbsp; 🧭 processo no <a href="./EQUIPE.md">manual do projeto</a></sub>

</div>

> **Nome antigo:** até 30/10/2026 o projeto se chamava **Lastro** ([ADR-009](./adr/009-nome-e-marca.md)).
> As levas daquela época ficam com o nome original; os links já apontam para o repositório novo.

---

## Por que este arquivo existe

Card arquivado some do board. A issue continua fechada e o commit continua no
histórico, mas a linha que liga *"o que foi combinado"* a *"o commit que
entregou"* só existe enquanto alguém lembra. Este arquivo guarda essa linha.

O board é a fonte da verdade do **agora**; este documento é a do **já foi**.

### Como arquivar

Ao fim da sprint, para cada card em `✅ Concluído`:

1. Confirme que a issue foi fechada pelo `Closes` no merge;
2. Ache o **PR que fechou** e o **release** que levou aquilo para a `main`;
3. Registre a linha na tabela da leva;
4. **Só então** arquive o card (`gh project item-archive <n> --owner vintemhq --id <id>`).

### Épico que fecha ganha seção própria

Épico não tem PR nem release. O que vale registrar dele é a leitura de
conjunto: o que era, o que virou, as decisões que deixou e o que ensinou.

---

## Leva de 30/10/2026 (fim da tarde) — v0.11.1: o primeiro Pix de verdade

O primeiro teste com dinheiro real, numa conta Efí Pro (CPF): uma cobrança de R$ 1,00 e outra de
R$ 10,00, pagas pelo Nubank, com o webhook, a conciliação e a devolução funcionando. Ele achou dois
defeitos que a homologação e o Efí falso não pegavam, e os dois estão corrigidos neste release.

| PR → `develop` | O que entrou | Release → `main` |
|---|---|---|
| [#168](https://github.com/vintemhq/vintem/pull/168) `22da710` | Devolução com descrição não é mais recusada pelo Efí | [v0.11.1](https://github.com/vintemhq/vintem/releases/tag/v0.11.1) · [#170](https://github.com/vintemhq/vintem/pull/170) `0e3b754` |
| [#169](https://github.com/vintemhq/vintem/pull/169) `04027c5` | Webhook do Efí pode ser registrado em produção | [v0.11.1](https://github.com/vintemhq/vintem/releases/tag/v0.11.1) · [#170](https://github.com/vintemhq/vintem/pull/170) `0e3b754` |

> Release [v0.11.1](https://github.com/vintemhq/vintem/releases/tag/v0.11.1) com binários, checksums e imagem `ghcr.io/vintemhq/vintem:0.11.1`, que respondeu `0.11.1`. O teste
> com o banco real rodou com o binário da branch do [#169](https://github.com/vintemhq/vintem/pull/169), no mesmo código.

### O webhook nunca teria funcionado

O Efí chama a URL durante o registro, e o Vintém só gravava o token depois de o banco responder. A
chamada recebia 401, e o registro falhava sempre. A homologação não pegou porque o webhook ficou
sem testar por falta de URL pública, e o Efí falso não chama a URL. O teste passou a reproduzir o
banco fazendo essa chamada.

### Como provar que foi o webhook

A consulta da cobrança pergunta ao banco, então um `CONCLUIDA` sozinho não prova nada. A prova foi a
ordem: não consultar antes, rodar `vintem conciliar` e ver `0 Pix recuperados`. No primeiro Pix, sem
webhook, a mesma conciliação tinha recuperado 1.

### A tarifa muda o que dá para devolver

O Efí cobrou R$ 0,01 num Pix de R$ 1,00 e R$ 0,12 num de R$ 10,00, cerca de 1,2%, e nada na devolução.
Por isso, devolver o valor integral de um Pix sem outro saldo na conta termina em `NAO_REALIZADO`, sem
motivo. Isso não é defeito do Vintém, mas quem integra precisa saber.

### Ficou aberto

- Cobrança com vencimento (`cobv`) só foi testada na homologação.
- O túnel temporário do teste não serve para produção, e a rede da UFLA bloqueia a porta dele.

---

## Leva de 30/10/2026 (tarde) — v0.11.0: o que a documentação mostrou que faltava

Escrever e testar as seções Conceitos e Operar mostrou problemas no produto, e esta leva os
corrige. A chave mestra agora é conferida ao abrir o banco e pode ser trocada, a API ganhou a
rota que a RN-41 prometia, e o `PATCH /cobr` parou de cancelar com qualquer status.

Nenhuma issue foi aberta para estas correções:

| PR → `develop` | O que entrou | Release → `main` |
|---|---|---|
| [#163](https://github.com/vintemhq/vintem/pull/163) `790e2aa` | Chave mestra diferente recusada ao abrir o banco, `vintem chave trocar` e aviso do `lastro.db` | [v0.11.0](https://github.com/vintemhq/vintem/releases/tag/v0.11.0) · [#166](https://github.com/vintemhq/vintem/pull/166) `abd1676` |
| [#164](https://github.com/vintemhq/vintem/pull/164) `efdf8ff` | `GET` e `PATCH /solicrec/{idSolicRec}`, e `PATCH /cobr` só cancela com `CANCELADA` | [v0.11.0](https://github.com/vintemhq/vintem/releases/tag/v0.11.0) · [#166](https://github.com/vintemhq/vintem/pull/166) `abd1676` |
| [#165](https://github.com/vintemhq/vintem/pull/165) `e02aab5` | Título próprio no erro de token do webhook e teste do comando do MCP | [v0.11.0](https://github.com/vintemhq/vintem/releases/tag/v0.11.0) · [#166](https://github.com/vintemhq/vintem/pull/166) `abd1676` |

> Release [v0.11.0](https://github.com/vintemhq/vintem/releases/tag/v0.11.0) com binários, checksums e imagem `ghcr.io/vintemhq/vintem:0.11.0`, conferida com o
> Compose da documentação, inclusive a troca da chave mestra com uma conta e um destino.

### Um PATCH que cancelava com qualquer status

`PATCH /cobr/{txid}` ignorava o corpo: `{"status":"CONCLUIDA"}` cancelava a cobrança. O teste que
deveria pegar isso rodava numa cobrança já cancelada, então passava pelo motivo errado. O defeito
apareceu ao escrever a rota irmã, `PATCH /solicrec`. Teste de recusa precisa partir de um estado em
que a operação teria efeito.

### Ficou aberto

- Pix Automático continua só no sandbox: `CancelarSolicitacao` não tem implementação em banco real,
  porque nenhum adapter declara `rec` ([#89](https://github.com/vintemhq/vintem/issues/89)).

---

## Leva de 30/10/2026 — v0.10.0: o site explica como rodar, e a primeira contribuição de fora

O site ganhou as seções Conceitos e Operar e as páginas de cada erro da API, e o
Compose de produção entrou no repositório. Veio a primeira contribuição externa do
projeto, de `jay23606`, e o manual passou a dizer como revisar um PR de fora.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#115](https://github.com/vintemhq/vintem/issues/115) | `vintem sandbox` imprime o comando para ligar o MCP no Claude Code | DX/Docs | P | Média | [#143](https://github.com/vintemhq/vintem/pull/143) `32778ec` | [v0.10.0](https://github.com/vintemhq/vintem/releases/tag/v0.10.0) · [#161](https://github.com/vintemhq/vintem/pull/161) `37beff8` | 30/10 10:04 |
| [#149](https://github.com/vintemhq/vintem/issues/149) | O link do campo type dos erros da API leva a uma página que existe | DX/Docs | M | Alta | [#156](https://github.com/vintemhq/vintem/pull/156) `0d55787` | [v0.10.0](https://github.com/vintemhq/vintem/releases/tag/v0.10.0) · [#161](https://github.com/vintemhq/vintem/pull/161) `37beff8` | 30/10 09:43 |
| [#147](https://github.com/vintemhq/vintem/issues/147) | Entender cobranças, webhooks e Pix Automático pelo site (seção Conceitos) | DX/Docs | G | Alta | [#159](https://github.com/vintemhq/vintem/pull/159) `e411184` | [v0.10.0](https://github.com/vintemhq/vintem/releases/tag/v0.10.0) · [#161](https://github.com/vintemhq/vintem/pull/161) `37beff8` | 30/10 10:26 |
| [#148](https://github.com/vintemhq/vintem/issues/148) | Instalar, configurar e operar o Vintém em produção pelo site (seção Operar) | DX/Docs | G | Alta | [#160](https://github.com/vintemhq/vintem/pull/160) `111c204` | [v0.10.0](https://github.com/vintemhq/vintem/releases/tag/v0.10.0) · [#161](https://github.com/vintemhq/vintem/pull/161) `37beff8` | 30/10 10:42 |

> **Também no release, sem fechar issue:** os certificados de teste ([#157](https://github.com/vintemhq/vintem/pull/157) `07de418`), o fluxo de
> PR externo no manual ([#158](https://github.com/vintemhq/vintem/pull/158) `66637a8`), o site em `vintem.dev` ([#144](https://github.com/vintemhq/vintem/pull/144) `baf9190`) e o SDK por
> Trusted Publishing ([#145](https://github.com/vintemhq/vintem/pull/145) `be62041` e [#146](https://github.com/vintemhq/vintem/pull/146) `55a8c2c`).
>
> Release [v0.10.0](https://github.com/vintemhq/vintem/releases/tag/v0.10.0) com binários, checksums e imagem `ghcr.io/vintemhq/vintem:0.10.0`, conferida
> com o Compose da documentação, inclusive na atualização a partir da 0.9.0.

### Um teste que dependia do dia

Dois testes dos bancos passaram a falhar em toda branch de um dia para o outro. O
certificado valia até `agora + 24h`, com `agora` fixo, mas o TLS confere no relógio real.
A varredura achou outros seis com o mesmo defeito, um deles passando pelo motivo errado.
Fica a regra, no comentário do `certificadotest.Gerar`: certificado que chega a um
handshake parte de `time.Now`.

### Testar a documentação achou erros nela

Rodar os comandos das páginas pegou um `PATCH /solicrec` que a API não tem, datas que a
janela do Pix Automático recusaria e um exemplo inventado, antes de irem para o site.
Também mostrou o que nenhum teste de unidade dizia: uma chave mestra diferente só aparece
na chamada ao banco, e um `lastro.db` não renomeado vira um `vintem.db` vazio sem aviso.

### Ficou aberto

- A issue do [#157](https://github.com/vintemhq/vintem/pull/157) não foi criada.

---

## Leva de 29/10/2026 (noite) — v0.9.0: o projeto passa a se chamar Vintém

O nome mudou antes do lançamento, pelo conflito de marca com a Lastro Tecnologia
registrado na [ADR-009](./adr/009-nome-e-marca.md). A troca quebra quem usava o alpha:
variáveis `LASTRO_*`, API keys `lst_`, banco padrão e caminho do `go install`.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#2](https://github.com/vintemhq/vintem/issues/2) | Saber se o nome Lastro está livre para uso no INPI e se o uso descritivo de "Pix" é aceito pelo manual de marca do BCB | Estudo | — | — | [#140](https://github.com/vintemhq/vintem/pull/140) `18c7381` | [v0.9.0](https://github.com/vintemhq/vintem/releases/tag/v0.9.0) · [#142](https://github.com/vintemhq/vintem/pull/142) `4aa14e2` | 29/10 22:37 |

> **Também no release, sem fechar issue:** o `-h` nos subcomandos ([#138](https://github.com/vintemhq/vintem/pull/138) `fa5a52e`), o estudo de
> marca ([#139](https://github.com/vintemhq/vintem/pull/139) `f0ed93a`) e a leva do v0.8.0 ([#141](https://github.com/vintemhq/vintem/pull/141) `2361f8d`).
>
> A imagem ficou pública depois deste release: [#98](https://github.com/vintemhq/vintem/issues/98) fechada em 29/10.

---

## Leva de 29/10/2026 (madrugada) — v0.8.0: quatro bancos sobre um cliente só

O adapter do Efí virou a configuração de um cliente da API Pix comum, e Inter,
Sicoob e Banco do Brasil entraram por cima dele. O Pix Automático teve as
transições conferidas nos diagramas do BCB. Foi o último release com o nome
Lastro.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#99](https://github.com/vintemhq/vintem/issues/99) | Transições da solicitação de confirmação conferidas no Manual de Fluxos do BCB | Estudo | P | Média | [#135](https://github.com/vintemhq/vintem/pull/135) `e2ca345` | [v0.8.0](https://github.com/vintemhq/vintem/releases/tag/v0.8.0) · [#137](https://github.com/vintemhq/vintem/pull/137) `270ffc5` | 29/10 21:27 |

> **Também no release, sem fechar issue:** o `pixapi` ([#131](https://github.com/vintemhq/vintem/pull/131) `1b3aea8`), os adapters do Inter
> ([#132](https://github.com/vintemhq/vintem/pull/132) `d9a0826`, parte da [#28](https://github.com/vintemhq/vintem/issues/28)), do Sicoob ([#133](https://github.com/vintemhq/vintem/pull/133) `e6b8072`, parte da [#29](https://github.com/vintemhq/vintem/issues/29))
> e do Banco do Brasil ([#134](https://github.com/vintemhq/vintem/pull/134) `fe0c36f`, parte da [#30](https://github.com/vintemhq/vintem/issues/30)), e a leva do v0.7.0
> ([#136](https://github.com/vintemhq/vintem/pull/136) `1cc3126`).
>
> Release [v0.8.0](https://github.com/vintemhq/vintem/releases/tag/v0.8.0) com binários e checksums; o binário
> linux/amd64 foi conferido contra o `checksums.txt`, respondeu `0.8.0` e o sandbox dele subiu.
>
> **Ainda não ⚠️ — imagem privada** ([#98](https://github.com/vintemhq/vintem/issues/98)).

### O manual estava num PDF que ninguém tinha aberto

A spec da API Pix lista os status do Pix Automático, mas não as transições. O
*Manual de Padrões para Iniciação do Pix* tem os diagramas, e três tabelas
estavam erradas: faltava ENVIADA → REJEITADA na solicitação e CRIADA →
CANCELADA na recorrência, e sobrava CRIADA → REJEITADA na cobrança recorrente.
A conferência achou também um bug: se o aviso de ATIVA se perdesse, o
CONCLUIDA que chegasse depois era ignorado para sempre. O status que vem do
banco passou a valer quando é alcançável no diagrama.

### Três bancos sem conta em nenhum

Inter, Sicoob e BB foram escritos pela documentação e por relatos públicos
(o kmee/brbank rodou os sandboxes do Sicoob e do BB), e passam na suíte contra
um banco simulado. Nenhum rodou ainda contra o banco de verdade, e os guias
dizem isso no topo.

### Ficou aberto

- [#28](https://github.com/vintemhq/vintem/issues/28), [#29](https://github.com/vintemhq/vintem/issues/29), [#30](https://github.com/vintemhq/vintem/issues/30) — teste manual com conta real.
- [#98](https://github.com/vintemhq/vintem/issues/98) — imagem pública no ghcr.io.

---

## Leva de 29/10/2026 (noite) — v0.7.0: o MVP de lançamento

O Lastro ficou pronto para ser mostrado: painel embutido no binário, servidor
MCP para agentes, SDK TypeScript, site de docs e um README que mostra em vez de
prometer.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#38](https://github.com/vintemhq/vintem/issues/38) | Criar e consultar cobranças a partir de um agente de IA (MCP) | DX/Docs | M | Média | [#112](https://github.com/vintemhq/vintem/pull/112) `93aa19b` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 19:27 |
| [#20](https://github.com/vintemhq/vintem/issues/20) | Subir o Lastro em modo sandbox com um comando | DX/Docs | P | Alta | [#113](https://github.com/vintemhq/vintem/pull/113) `0659af0` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 19:40 |
| [#34](https://github.com/vintemhq/vintem/issues/34) | Tokens e componentes base do design system no painel | Design | M | Média | [#126](https://github.com/vintemhq/vintem/pull/126) `5547f65` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 20:01 |
| [#35](https://github.com/vintemhq/vintem/issues/35) | Ver cobranças, Pix recebidos e quanto economizei no painel | Painel | G | Média | [#126](https://github.com/vintemhq/vintem/pull/126) `5547f65` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 20:01 |
| [#36](https://github.com/vintemhq/vintem/issues/36) | Ver e reenviar webhooks que falharam pelo painel | Painel | G | Média | [#127](https://github.com/vintemhq/vintem/pull/127) `adad0bc` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 20:12 |
| [#39](https://github.com/vintemhq/vintem/issues/39) | Site de docs em lastro.dev | DX/Docs | G | Média | [#128](https://github.com/vintemhq/vintem/pull/128) `bc8b691` | [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) · [#130](https://github.com/vintemhq/vintem/pull/130) `92bc1aa` | 29/10 20:19 |

> **Também no release, sem fechar issue:** o SDK `@vintemhq/sdk` da [#37](https://github.com/vintemhq/vintem/issues/37)
> ([#125](https://github.com/vintemhq/vintem/pull/125) `a694820`), o README de lançamento da [#40](https://github.com/vintemhq/vintem/issues/40) (no [#113](https://github.com/vintemhq/vintem/pull/113))
> e a leva do v0.6.0 ([#129](https://github.com/vintemhq/vintem/pull/129) `aa4d791`).
>
> Release [v0.7.0](https://github.com/vintemhq/vintem/releases/tag/v0.7.0) com binários e checksums.
>
> **Ainda não ⚠️ — imagem privada** ([#98](https://github.com/vintemhq/vintem/issues/98)).

### O nome do pacote já tinha dono

O escopo `@lastro` no npm pertence a outra pessoa. O SDK virou `@vintemhq/sdk`,
igual à organização do GitHub, e a publicação espera a organização no npm e o
`NPM_TOKEN` ([#37](https://github.com/vintemhq/vintem/issues/37)).

### Ficou aberto

- [#37](https://github.com/vintemhq/vintem/issues/37) — publicar o SDK no npm.
- [#40](https://github.com/vintemhq/vintem/issues/40) — social preview em Settings.
- [#41](https://github.com/vintemhq/vintem/issues/41) — GitHub Sponsors.
- [#98](https://github.com/vintemhq/vintem/issues/98) — imagem pública no ghcr.io.

---

## Leva de 29/10/2026 (noite) — v0.6.0: Pix Automático completo, devoluções e o primeiro banco de verdade respondendo

A cobrança recorrente fechou a F6 e as devoluções abriram a F7, as duas no
sandbox. A suíte de conformidade ganhou fitas gravadas na homologação real do
Efí, e a gravação achou três desvios que o servidor falso escondia. O sandbox
passou a subir com um comando.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#32](https://github.com/vintemhq/vintem/issues/32) | Cobrar automaticamente as parcelas de uma recorrência aprovada | Core | G | Alta | [#100](https://github.com/vintemhq/vintem/pull/100) `88afe10`, [#101](https://github.com/vintemhq/vintem/pull/101) `2c0cc2f`, [#102](https://github.com/vintemhq/vintem/pull/102) `5daa183`, [#103](https://github.com/vintemhq/vintem/pull/103) `ef75558` | [v0.6.0](https://github.com/vintemhq/vintem/releases/tag/v0.6.0) · [#111](https://github.com/vintemhq/vintem/pull/111) `ba6ad2c` | 29/10 17:07 |
| [#33](https://github.com/vintemhq/vintem/issues/33) | Devolver um Pix total ou parcialmente sem passar do valor recebido | Core | M | Média | [#104](https://github.com/vintemhq/vintem/pull/104) `e526e19`, [#105](https://github.com/vintemhq/vintem/pull/105) `f41dce4`, [#106](https://github.com/vintemhq/vintem/pull/106) `82f8067` | [v0.6.0](https://github.com/vintemhq/vintem/releases/tag/v0.6.0) · [#111](https://github.com/vintemhq/vintem/pull/111) `ba6ad2c` | 29/10 17:41 |
| [#21](https://github.com/vintemhq/vintem/issues/21) | Todo adapter passa pela mesma suíte de conformidade | Adapters PSP | G | Alta | [#108](https://github.com/vintemhq/vintem/pull/108) `14cdf3e` | [v0.6.0](https://github.com/vintemhq/vintem/releases/tag/v0.6.0) · [#111](https://github.com/vintemhq/vintem/pull/111) `ba6ad2c` | 29/10 18:46 |

> **Também no release, sem fechar issue:** o `lastro sandbox` da [#20](https://github.com/vintemhq/vintem/issues/20)
> ([#107](https://github.com/vintemhq/vintem/pull/107) `d965f7c`), as fitas de pagamento e devolução do Efí da [#27](https://github.com/vintemhq/vintem/issues/27)
> ([#109](https://github.com/vintemhq/vintem/pull/109) `f709a32`) e a leva do v0.5.0 ([#110](https://github.com/vintemhq/vintem/pull/110) `0b45367`).
>
> Release [v0.6.0](https://github.com/vintemhq/vintem/releases/tag/v0.6.0) com 6 binários e checksums; o binário
> linux/amd64 foi conferido contra o `checksums.txt`, respondeu `0.6.0`, e o `lastro sandbox` dele levou uma
> cobrança a `CONCLUIDA`.
>
> **Ainda não ⚠️ — imagem privada** ([#98](https://github.com/vintemhq/vintem/issues/98)).

### A homologação contou o que a documentação não contava

A gravação contra o Efí real achou: `PUT /v2/cobv/{txid}` com txid existente
**sobrescreve** a cobrança em silêncio (o adapter passou a consultar antes), a
location da cobv só vem em `loc.location`, e devolução inexistente responde 400
em vez de 404. O servidor falso passou a imitar os três, para a suíte local
pegar uma regressão. A primeira gravação também trouxe nome, CPF e endereço do
dono da conta no campo `recebedor`; a auditoria pegou antes do commit, e a fita
passou a mascarar recebedor e pagador por padrão.

### "0% fees" saiu do README

A pesquisa de taxas para o README mostrou que a Efí Empresas cobra 1,19% por Pix
pela API, a mesma taxa da Stripe. O título deixou de prometer taxa zero e passou
a dizer o que é verdade: o Lastro não soma nada à tarifa do banco.

### Ficou aberto

- [#20](https://github.com/vintemhq/vintem/issues/20) — GIF do README (fechou depois, no v0.7.0).
- [#27](https://github.com/vintemhq/vintem/issues/27) — cadastro do webhook do Efí, que pede URL pública.
- [#98](https://github.com/vintemhq/vintem/issues/98) — imagem pública no ghcr.io.

---

## Leva de 29/10/2026 (fim da tarde) — v0.5.0: Pix Automático de ponta a ponta, por enquanto no sandbox

A F6 começou: recorrência criada, pedido de autorização ao pagador, aviso de
aprovação, rejeição ou expiração pelo webhook de recorrência, e a conciliação
recuperando o aviso que se perdeu. Tudo isso só no sandbox, porque nenhum banco
real deixa testar Pix Automático sem conta PJ. E as contas cadastradas antes do
v0.3.0 passaram a receber o aviso de vencimento do certificado.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#79](https://github.com/vintemhq/vintem/issues/79) | Contas cadastradas antes do v0.3.0 também recebem o aviso de vencimento do certificado | Segurança | P | Média | [#87](https://github.com/vintemhq/vintem/pull/87) `15c6d9b` | [v0.5.0](https://github.com/vintemhq/vintem/releases/tag/v0.5.0) · [#96](https://github.com/vintemhq/vintem/pull/96) `96736cb` | 29/10 14:18 |
| [#1](https://github.com/vintemhq/vintem/issues/1) | Saber quais bancos expõem /rec e /cobr (Pix Automático) em produção para PJ | Estudo | M | Alta | — (estudo) | [v0.5.0](https://github.com/vintemhq/vintem/releases/tag/v0.5.0) · [#96](https://github.com/vintemhq/vintem/pull/96) `96736cb` | 29/10 14:46 |
| [#31](https://github.com/vintemhq/vintem/issues/31) | Criar recorrência de Pix Automático e pedir autorização ao pagador | Core | G | Alta | [#90](https://github.com/vintemhq/vintem/pull/90) `054cc34`, [#91](https://github.com/vintemhq/vintem/pull/91) `ea30346`, [#92](https://github.com/vintemhq/vintem/pull/92) `d0145c0`, [#94](https://github.com/vintemhq/vintem/pull/94) `deda649` | [v0.5.0](https://github.com/vintemhq/vintem/releases/tag/v0.5.0) · [#96](https://github.com/vintemhq/vintem/pull/96) `96736cb` | 29/10 15:20 |
| [#93](https://github.com/vintemhq/vintem/issues/93) | Recorrência aprovada sem aviso do banco aparece no Lastro (conciliação de recorrências) | Core | M | Alta | [#95](https://github.com/vintemhq/vintem/pull/95) `8a9c095` | [v0.5.0](https://github.com/vintemhq/vintem/releases/tag/v0.5.0) · [#96](https://github.com/vintemhq/vintem/pull/96) `96736cb` | 29/10 15:31 |

> Release [v0.5.0](https://github.com/vintemhq/vintem/releases/tag/v0.5.0) com 6 binários e checksums; o workflow
> `Release` passou e publicou `ghcr.io/vintemhq/vintem:0.5.0`. O binário linux/amd64 foi baixado, conferido contra o
> `checksums.txt` e respondeu `0.5.0`.
>
> **Ainda não ⚠️ — imagem privada** ([#98](https://github.com/vintemhq/vintem/issues/98)): por isso o digest da imagem
> não foi conferido nesta leva.

### O estudo antes do código

A #1 decidiu a ordem da F6: as RN-40 a RN-43 foram revisadas contra a API Pix
2.10.0 e o Guia do Pix Automático v1.3 antes de qualquer tabela existir. O
resultado também disse o que não dava para fazer ainda: o Efí só libera `/rec`
para conta PJ, o que virou a [#89](https://github.com/vintemhq/vintem/issues/89).

### O sandbox faz o papel do pagador

Sem banco real para testar, o sandbox responde pelo pagador: aprova, rejeita
(R$ 0,77) ou deixa expirar (R$ 0,88) conforme o valor fixo da recorrência, e
avisa pelo webhook de recorrência. A conciliação (#93) existe porque aviso de
banco se perde; ela consulta no PSP cada recorrência ainda aberta e aplica o que
mudou pelo mesmo caminho do webhook.

### Ficou aberto

- [#32](https://github.com/vintemhq/vintem/issues/32) — cobrança recorrente (`/cobr`), a segunda metade do Pix Automático.
- [#33](https://github.com/vintemhq/vintem/issues/33) — devoluções (F7).
- [#89](https://github.com/vintemhq/vintem/issues/89) — conta PJ para testar Pix Automático num banco real.
- [#27](https://github.com/vintemhq/vintem/issues/27) e [#21](https://github.com/vintemhq/vintem/issues/21) — teste do Efí na homologação e fixtures gravadas.

---

## Leva de 29/10/2026 (tarde) — v0.4.0: o primeiro banco de verdade, ainda sem falar com ele

O Lastro ganhou o adapter do Efí: cob, cobv, Pix recebidos, devolução e webhook,
com mTLS e OAuth2 por conta, escrito pela documentação oficial e passando na
suíte de conformidade contra um Efí simulado. E o sandbox parou de esquecer as
cobranças quando o processo reinicia.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#64](https://github.com/vintemhq/vintem/issues/64) | Sandbox lembra das cobranças depois de reiniciar | Adapters PSP | M | Baixa | [#85](https://github.com/vintemhq/vintem/pull/85) `49bedce` | [v0.4.0](https://github.com/vintemhq/vintem/releases/tag/v0.4.0) · [#83](https://github.com/vintemhq/vintem/pull/83) `62266ca` | 29/10 13:51 |

> **Também no release, sem fechar issue:** a [#27](https://github.com/vintemhq/vintem/issues/27) (adapter do Efí) entrou
> em três PRs — [#80](https://github.com/vintemhq/vintem/pull/80) `169ca43`, [#81](https://github.com/vintemhq/vintem/pull/81) `92037b3`
> e [#82](https://github.com/vintemhq/vintem/pull/82) `f8c5a5d` — e segue em `🧪 Em teste` até o teste na homologação do Efí, que é
> critério de aceite. A correção dos testes instáveis entrou pelo [#84](https://github.com/vintemhq/vintem/pull/84) `b445ed7`.
>
> Release [v0.4.0](https://github.com/vintemhq/vintem/releases/tag/v0.4.0) com 6 binários, checksums e imagem
> `ghcr.io/vintemhq/vintem:0.4.0` (amd64 e arm64, digest `sha256:aba25d53…`). O binário linux/amd64 foi baixado,
> conferido contra o `checksums.txt`, respondeu `0.4.0` e lista `[efi sandbox]`.
>
> **Ainda não ⚠️ — imagem privada**, a mesma pendência das levas anteriores.

### Um banco escrito pela documentação

Sem credenciais de homologação, o adapter do Efí foi escrito pela documentação
oficial e testado contra um Efí simulado que segue os exemplos dela: TLS que exige
certificado de cliente, token por Basic auth com corpo JSON, erros no formato
próprio do Efí. A suíte de conformidade pegou dois erros, nenhum do adapter: o
simulado deixava pagar cobrança removida, e o exemplo de aviso da documentação
tem um `endToEndId` de 26 caracteres, quando a spec exige 32. **A validação
ficou; o exemplo é que estava errado.** O único ponto baseado em suposição (o
erro de `txid` repetido na cobv) está marcado no código e no roteiro da #27.

### Corrigir um arquivo não é corrigir o defeito

O CI do release reprovou num teste de API key que falhava 1 vez em 64: o mesmo
defeito corrigido no v0.3.0 (trocar o último caractere de um token sorteado por
`x`, que às vezes já era `x`). No v0.3.0 só o arquivo que falhou foi corrigido. A
busca pelo padrão achou mais um, e uma busca mais larga, outro com outra forma.
Ficou a regra: **achou um defeito, procura o padrão no repositório inteiro.**

### Reconstruir em vez de guardar

A #64 oferecia duas saídas: tabelas próprias para o sandbox ou reconstruir o
estado pelas cobranças gravadas. A segunda ganhou porque o que o sandbox sabia já
era o que o Lastro grava. O teste que prova isso reproduz o defeito sem a mudança
("0 Pix recuperados") e passa com ela; o binário de verdade foi parado antes do
pagamento simulado e, ao voltar, a cobrança apareceu paga.

### Ficou aberto

- [#27](https://github.com/vintemhq/vintem/issues/27) — teste do adapter do Efí na homologação; roteiro de 7 passos na issue.
- [#79](https://github.com/vintemhq/vintem/issues/79) — contas cadastradas antes do v0.3.0 não recebem o aviso de vencimento do certificado.
- [#21](https://github.com/vintemhq/vintem/issues/21) — fixtures gravadas da homologação para a suíte de conformidade.
- [#1](https://github.com/vintemhq/vintem/issues/1) — estudo de quais bancos expõem Pix Automático, que destrava a F6.

---

## Leva de 29/10/2026 — v0.3.0: o certificado é conferido antes de o banco reclamar

A base dos adapters reais (F5), entregue antes de o primeiro banco entrar: o
certificado do mTLS de cada conta é lido no cadastro, vigiado até vencer e
preso ao ambiente certo; o token OAuth2 é reutilizado; e a regra de camadas
virou check do CI. O BR Code saiu de `🧪 Em teste` com o teste manual em quatro apps.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#9](https://github.com/vintemhq/vintem/issues/9) | Gerar BR Code estático e dinâmico válido em qualquer app de banco | Core | M | Alta | [#67](https://github.com/vintemhq/vintem/pull/67) `832b45e` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 10:55 |
| [#4](https://github.com/vintemhq/vintem/issues/4) | CI reprova importação proibida entre camadas (depguard) | Infra/CI | P | Média | [#68](https://github.com/vintemhq/vintem/pull/68) `c7fa073` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 11:08 |
| [#26](https://github.com/vintemhq/vintem/issues/26) | Token OAuth2 dos bancos é reutilizado até expirar | Core | M | Alta | [#70](https://github.com/vintemhq/vintem/pull/70) `68ecaa7` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 11:53 |
| [#71](https://github.com/vintemhq/vintem/issues/71) | Certificado do mTLS é conferido ao cadastrar e a validade fica visível | Segurança | M | Alta | [#74](https://github.com/vintemhq/vintem/pull/74) `d168f1a` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 12:11 |
| [#73](https://github.com/vintemhq/vintem/issues/73) | Certificado de homologação é recusado em conta de produção | Segurança | P | Alta | [#75](https://github.com/vintemhq/vintem/pull/75) `cc1e36c` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 12:25 |
| [#72](https://github.com/vintemhq/vintem/issues/72) | A aplicação é avisada 30 dias antes de o certificado vencer | Segurança | M | Alta | [#76](https://github.com/vintemhq/vintem/pull/76) `cfd6ba1` | [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) · [#77](https://github.com/vintemhq/vintem/pull/77) `475e2aa` | 29/10 12:28 |

> `develop` e `main` com o mesmo conteúdo depois do [#77](https://github.com/vintemhq/vintem/pull/77); release
> [v0.3.0](https://github.com/vintemhq/vintem/releases/tag/v0.3.0) com 6 binários, checksums e imagem
> `ghcr.io/vintemhq/vintem:0.3.0` (amd64 e arm64, digest `sha256:9fa67e6b…`). O binário linux/amd64
> foi baixado, conferido contra o `checksums.txt` e respondeu `0.3.0`.
>
> **Ainda não ⚠️ — imagem privada**, a mesma pendência das levas anteriores.

### Épico #25 — certificados e segredos

**Era:** "certificados e segredos ficam cifrados e o painel avisa antes de
vencerem". Metade já existia (vault, master key obrigatória); a outra metade
dependia de uma coisa que não existia: o Lastro **ler** o certificado.

**Virou** três issues, um PR cada: conferir no cadastro (#71), recusar
homologação em produção (#73) e avisar a 30 dias (#72). O alerta no painel
fica para a F8.

**Decisões:** `go-pkcs12` da SSLMate, porque o `x/crypto/pkcs12` não lê .p12 com
AES, que é o padrão do OpenSSL 3 ([ADR-006](./adr/006-certificado-conferido-no-cadastro.md)).
"Certificado de homologação" passou a ser **o já cadastrado numa conta de
homologação**, pela impressão digital, porque nada no X.509 marca o ambiente.
O aviso é guardado pela impressão digital, para que trocar o certificado o zere
sem código extra.

**Ensinou:** a validade de certificado pode ser 9999-12-31, que não cabe em
nanossegundos `int64`. Por isso essa coluna é a única em segundos.

### Teste que só passa por sorte não é teste

Repetir a suíte 15 vezes antes de abrir o #74 revelou um teste antigo que falhava
1 vez em 64: ele "invalidava" o token trocando o último caractere por `x`, e
às vezes o token já terminava em `x`. Corrigido em commit separado. Ficou a regra:
**teste novo roda em laço antes do PR.**

### O `-race` achou o que já estava lá

O job de certificado, o primeiro ligado por padrão no teste que sobe o `serve`,
expôs que o `serve` nunca esperou os jobs antes de fechar o store. Conciliação e
entrega sempre tiveram o defeito; o teste os desligava. O `fix:` foi junto do #76.

### Prova de regra de lint precisa de import de verdade

O PR descartável que provou o depguard (#69) falhou pela razão errada duas vezes:
primeiro um ciclo de import, depois um import em branco que o `revive` reclamou na
mesma linha — e o golangci-lint mostra um problema por linha. Só com um import
usado de verdade o depguard apareceu.

### Ficou aberto

- [#27](https://github.com/vintemhq/vintem/issues/27) — adapter do Efí, que usa tudo o que entrou aqui; espera acesso à homologação.
- [#21](https://github.com/vintemhq/vintem/issues/21) — conformidade espera as fixtures do primeiro banco real.
- [#64](https://github.com/vintemhq/vintem/issues/64) — sandbox ainda esquece as cobranças ao reiniciar.
- [#1](https://github.com/vintemhq/vintem/issues/1) — estudo de quais bancos expõem Pix Automático, que destrava a F6.
- [#79](https://github.com/vintemhq/vintem/issues/79) — contas cadastradas antes do v0.3.0 não têm os dados públicos do certificado e não recebem o aviso de vencimento.

---

## Leva de 28/10/2026 (noite) — v0.2.0: nenhum Pix some

A F4 inteira numa leva: o Lastro passou a **saber** do pagamento (webhook do
banco ou conciliação), gravá-lo uma vez só e **avisar** a aplicação do cliente
com assinatura verificável. Junto, o projeto foi para o Go 1.27.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#59](https://github.com/vintemhq/vintem/issues/59) | Projeto compila e roda no Go 1.27 | Infra/CI | P | Média | [#60](https://github.com/vintemhq/vintem/pull/60) `6aad7e3` | [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) · [#65](https://github.com/vintemhq/vintem/pull/65) `12d2c40` | 28/10 22:13 |
| [#22](https://github.com/vintemhq/vintem/issues/22) | Webhook do banco é aceito, deduplicado e persistido antes do 200 | Core | G | Alta | [#61](https://github.com/vintemhq/vintem/pull/61) `6103f66` | [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) · [#65](https://github.com/vintemhq/vintem/pull/65) `12d2c40` | 28/10 22:22 |
| [#24](https://github.com/vintemhq/vintem/issues/24) | Pix recebido sem webhook aparece em até 5 minutos (conciliação) | Core | M | Alta | [#62](https://github.com/vintemhq/vintem/pull/62) `6ef5174` | [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) · [#65](https://github.com/vintemhq/vintem/pull/65) `12d2c40` | 28/10 22:28 |
| [#23](https://github.com/vintemhq/vintem/issues/23) | A aplicação do cliente recebe webhook assinado mesmo se estiver fora do ar por horas | Core | G | Alta | [#63](https://github.com/vintemhq/vintem/pull/63) `48647f9` | [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) · [#65](https://github.com/vintemhq/vintem/pull/65) `12d2c40` | 28/10 22:39 |
| [#19](https://github.com/vintemhq/vintem/issues/19) | Testar o fluxo completo de Pix sem conta PJ (sandbox com estado) | Adapters PSP | G | Alta | — (fechada com evidência de #51, #55, #62 e #63) | [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) · [#65](https://github.com/vintemhq/vintem/pull/65) `12d2c40` | 28/10 22:39 |

> `develop` e `main` com o mesmo conteúdo depois do [#65](https://github.com/vintemhq/vintem/pull/65); release
> [v0.2.0](https://github.com/vintemhq/vintem/releases/tag/v0.2.0) com 6 binários, checksums e imagem `ghcr.io/vintemhq/vintem:0.2.0`.
>
> **Ainda não ⚠️ — imagem privada**, a mesma pendência da leva do v0.1.0.

### Uma peça, três portas

As três issues da F4 eram, no fundo, uma só: **registrar um Pix recebido** de
forma idempotente pelo `endToEndId`, concluindo a cobrança e publicando os
eventos na mesma transação. O webhook (#22) e a conciliação (#24) são duas
entradas para `Recebimentos.Registrar`; a entrega (#23) consome o que ele publica.
Desenhar a peça primeiro fez a #24 sair em uma fração do tempo da #22.

### A prova que vale é a de fora

A assinatura do Standard Webhooks foi implementada pelo Lastro e **verificada pela
biblioteca oficial** nos testes — assinar e verificar com a mesma lib não provaria
nada. No ponta a ponta, um receptor em Python verificou de novo, de forma
independente, enquanto recusava de propósito as duas primeiras entregas.

### O `tail` escondeu uma falha

Na conciliação, um teste pareceu passar quando os dois tinham falhado: o
`tail -5` cortou a saída. A causa real era o Pix cair exatamente em `agora`, fora
da janela semiaberta — comportamento certo, que a rodada seguinte cobre. Ficou a
regra: **saída de teste se lê inteira.**

### O gzip mudou de ideia

Subir para o Go 1.27 regenerou a spec embutida no código do oapi-codegen: o
compressor da 1.27 produz bytes diferentes para o mesmo conteúdo. O check de
drift do CI pegaria isso num PR distraído — é para isso que ele existe.

### Ficou aberto

- [#64](https://github.com/vintemhq/vintem/issues/64) — sandbox ainda esquece as cobranças ao reiniciar (tirado da #19).
- [#21](https://github.com/vintemhq/vintem/issues/21) — conformidade espera as fixtures do primeiro banco real (F5).
- [#9](https://github.com/vintemhq/vintem/issues/9) — BR Code ainda sem teste manual em apps de banco.
- [#1](https://github.com/vintemhq/vintem/issues/1) — estudo de quais bancos expõem Pix Automático, que destrava a F6.

---

## Leva de 28/10/2026 — v0.1.0: do repositório vazio à primeira cobrança paga no sandbox

O primeiro release, e o dia inteiro dele: **F0, F1 e F2 fecharam juntas**. O
Lastro saiu de um PDF de ideias para um binário que cria cob e cobv pela API
compatível com o BCB, autentica por API key, deduplica por Idempotency-Key e
publica imagem multi-arch no `ghcr.io`.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#3](https://github.com/vintemhq/vintem/issues/3) | main e develop protegidas com o check Lint & Test | Infra/CI | P | Alta | — (via API, comentado na issue) | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 18:24 |
| [#5](https://github.com/vintemhq/vintem/issues/5) | Release com tag publica binários e imagem multi-arch no ghcr.io | Infra/CI | M | Média | [#56](https://github.com/vintemhq/vintem/pull/56) `8695c28` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 21:49 |
| [#6](https://github.com/vintemhq/vintem/issues/6) | Valores em dinheiro nunca perdem centavo (Money em int64) | Core | P | Alta | [#44](https://github.com/vintemhq/vintem/pull/44) `9ea705b` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 18:54 |
| [#7](https://github.com/vintemhq/vintem/issues/7) | txid gerado e validado conforme a spec (26–35 alfanuméricos) | Core | P | Alta | [#44](https://github.com/vintemhq/vintem/pull/44) `9ea705b` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 18:54 |
| [#8](https://github.com/vintemhq/vintem/issues/8) | Cobrança só muda de estado por transição permitida | Core | M | Alta | [#45](https://github.com/vintemhq/vintem/pull/45) `2b4c046` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 18:57 |
| [#10](https://github.com/vintemhq/vintem/issues/10) | Ler um BR Code "Pix Copia e Cola" e saber o que ele cobra | Core | M | Média | [#46](https://github.com/vintemhq/vintem/pull/46) `0b30133` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 19:00 |
| [#11](https://github.com/vintemhq/vintem/issues/11) | Ports PSP, Store, Notifier, Vault e Clock definidos | Core | M | Alta | [#47](https://github.com/vintemhq/vintem/pull/47) `bffc2aa` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 19:04 |
| [#12](https://github.com/vintemhq/vintem/issues/12) | Dados do Lastro persistem em SQLite com migrations versionadas | Core | M | Alta | [#48](https://github.com/vintemhq/vintem/pull/48) `717b6e5` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 20:04 |
| [#13](https://github.com/vintemhq/vintem/issues/13) | Contrato da API em api/openapi.yaml derivado da spec do BCB | API | M | Alta | [#49](https://github.com/vintemhq/vintem/pull/49) `f472593` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 20:23 |
| [#18](https://github.com/vintemhq/vintem/issues/18) | Uma instância atende várias contas recebedoras | Core | M | Alta | [#50](https://github.com/vintemhq/vintem/pull/50) `188ca42` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 20:34 |
| [#14](https://github.com/vintemhq/vintem/issues/14) | Criar e consultar cobrança imediata (cob) pela API | API | G | Alta | [#52](https://github.com/vintemhq/vintem/pull/52) `656ad18` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 20:49 |
| [#16](https://github.com/vintemhq/vintem/issues/16) | Acesso à API só com API key de escopo limitado | Segurança | M | Alta | [#53](https://github.com/vintemhq/vintem/pull/53) `7ac80e2` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 20:58 |
| [#17](https://github.com/vintemhq/vintem/issues/17) | Reenviar a mesma criação não gera cobrança duplicada (Idempotency-Key) | API | M | Alta | [#54](https://github.com/vintemhq/vintem/pull/54) `2582256` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 21:05 |
| [#15](https://github.com/vintemhq/vintem/issues/15) | Criar e consultar cobrança com vencimento (cobv) pela API | API | G | Média | [#55](https://github.com/vintemhq/vintem/pull/55) `ed0eec2` | [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) · [#57](https://github.com/vintemhq/vintem/pull/57) `7a0a897` | 28/10 21:23 |

> `develop` e `main` com o mesmo conteúdo depois do [#57](https://github.com/vintemhq/vintem/pull/57);
> release [v0.1.0](https://github.com/vintemhq/vintem/releases/tag/v0.1.0) com 6 binários, checksums e imagem
> `ghcr.io/vintemhq/vintem:0.1.0` (amd64 e arm64).
>
> **Ainda não ⚠️ — imagem privada.** Pacote novo no `ghcr.io` nasce privado, e a API
> do GitHub não muda visibilidade de pacote: é na interface da org. Até lá,
> `docker pull` anônimo responde `unauthorized`. Atualizar esta linha quando abrir.

### O contrato pegou o que o teste de unidade não pegava

A decisão de **validar toda resposta HTTP contra o `openapi.yaml`** (kin-openapi)
pagou duas vezes no mesmo dia: header obrigatório ausente respondia `text/plain`
em vez de `Problema` (#14), e a ausência de uma resposta `default` teria escondido
os 502 de PSP do contrato. Nenhum teste de unidade olhava para isso.

### Três desvios da spec, corrigidos antes de sair

Conferir a spec do BCB, e não a memória, mudou três regras já escritas:
`revisao` começa em **0**, a expiração padrão da cob é **86400 s** (não 3600) e o
**CNPJ é alfanumérico**. Viraram um commit `fix:` separado e notas na RN-13/RN-14.

### O CI que nunca rodou

O primeiro PR ficou sem check: o workflow só existia nas branches de feature, e a
`develop` protegida exigia um check que nunca aparecia. Saiu com **um desvio único
autorizado** — proteção desligada, merge do #43, proteção religada no mesmo
comando. Fica registrado para não repetir: **workflow de CI entra na branch padrão
antes de ela ser protegida.**

### Go 1.26.0 tinha 23 vulnerabilidades

`go 1.26.0` no `go.mod` fazia o `setup-go` instalar exatamente o `.0`. O
`govulncheck` reprovou o #48, e o CI passou a usar o último patch (`1.26.x`), com
`toolchain go1.26.10` no `go.mod` para quem roda local.

### Ficou aberto

- [#19](https://github.com/vintemhq/vintem/issues/19) — sandbox ainda não dispara webhook nem persiste estado (F4).
- [#21](https://github.com/vintemhq/vintem/issues/21) — suíte de conformidade espera as fixtures do primeiro banco real (F5).
- [#9](https://github.com/vintemhq/vintem/issues/9) — BR Code ainda sem teste manual em apps de banco.

---

<!-- Modelo de leva:

## Leva de DD/MM/AAAA — título que diz o que mudou

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|

-->

