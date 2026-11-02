# 📜 Vintém — Regras de negócio

O que o Vintém **garante**, o que ele **recusa** e por quê. Toda regra tem um
número estável (`RN-xx`) para ser citada em issue, teste e commit. Regra que
muda ganha nota com data; regra que cai fica riscada, não some.

> Os nomes de estado e campo seguem a [spec da API Pix do BCB](https://github.com/bacen/pix-api)
> (v2.10.0). Onde o Vintém estende a spec, isso está dito explicitamente.

---

## Princípios

### RN-01 · Não custodial

O Vintém nunca guarda, recebe nem movimenta dinheiro. Ele roda na
infraestrutura do dono da conta, com as credenciais do dono da conta, contra a
API Pix do banco do dono da conta. Nenhuma funcionalidade pode exigir que o
dinheiro passe por uma conta que não seja a do recebedor.

**Por quê:** é o que mantém o projeto fora do papel de instituição de pagamento.
A tese de que software não custodial dispensa autorização do BCB é uma
**inferência**, não norma encontrada — por isso o README traz o aviso de que
nada aqui é aconselhamento regulatório, e uma eventual versão hospedada exige
análise jurídica própria antes de existir.

### RN-02 · Dinheiro é inteiro, em centavos

Valores trafegam e são armazenados como `int64` em centavos (`domain.Money`).
`float` não aparece em nenhuma camada. A spec do BCB usa string decimal no
formato `^\d{1,10}\.\d{2}$` (`"10.50"`); a conversão acontece só na borda
(adapter e API) e aceita **exatamente** esse formato — sem sinal, vírgula,
separador de milhar ou casas a mais ou a menos. Máximo: R$ 9.999.999.999,99.

### RN-03 · Uma instância, N contas

Uma instância do Vintém atende várias **contas recebedoras**. Conta = PSP +
chave Pix + credenciais (client id/secret e certificado) + ambiente
(homologação ou produção). Toda cobrança pertence a exatamente uma conta
(chave estrangeira no banco).

- O ID da conta é `[a-z0-9-]`, até 63 caracteres, e não muda. O PSP também não.
- Conta nasce ativa. **Conta desativada recusa novas cobranças**; as existentes
  continuam valendo e recebendo webhook.
- O cadastro é pela CLI (`vintem conta criar|listar|desativar`). As rotas
  `/vintem/v1/contas` entram junto com as API keys (#16), com o escopo `conta:admin`.
- Segredos vêm de **arquivo**, nunca de flag, porque flag fica no histórico do shell.

---

## Cobranças

### RN-10 · txid

- Em `cob`, `cobv` e `cobr`: de **26 a 35** caracteres, `[a-zA-Z0-9]`.
- Se o cliente não informar, o Vintém gera (aleatório criptográfico, 32 caracteres).
- Se informar, o Vintém valida antes de chamar o banco — erro de formato nunca
  vira chamada de rede.
- txid é único **na instância inteira** — mais estrito que a spec (que exige
  unicidade por recebedor), para que `GET /cob/{txid}` não precise saber a conta.
  Vale entre os três tipos: o txid de uma cob não serve para uma cobv nem para uma cobr.
- A conta da cobrança é a dona da `chave` informada no corpo. Chave Pix é única
  na instância, como no DICT.

### RN-11 · Idempotência

Toda criação ou alteração (`POST`, `PUT`, `PATCH`) aceita o header `Idempotency-Key`.

- A chave pertence à **API key** que a enviou: clientes diferentes não colidem.
- Mesma chave + mesma requisição (método, caminho e corpo) em até **24 h** →
  devolve a resposta original, byte a byte, com `Idempotent-Replayed: true`, sem
  chamar o banco de novo.
- Mesma chave + requisição diferente → **409** `IdempotencyKeyReutilizada`.
- Duas requisições simultâneas com a mesma chave → a segunda recebe **409**
  `RequisicaoEmAndamento` com `Retry-After`. Reserva sem resposta por mais de
  **2 minutos** é considerada abandonada (processo que caiu) e liberada.
- Resposta **5xx não é guardada**: o cliente mandou a chave justamente para
  poder tentar de novo.
- O header é obrigatório onde o identificador é gerado pelo servidor (`POST /cob`,
  `POST /rec`, `POST /solicrec` e `POST /cobr`) e opcional nos `PUT` com txid
  (`/cob`, `/cobv` e `/cobr`), em que o txid já identifica a criação.
- Corpo acima de 1 MiB com a chave → **413**.

### RN-12 · Estados da cobrança (`cob` e `cobv`)

```
ATIVA ──▶ CONCLUIDA
  │
  ├──▶ REMOVIDA_PELO_USUARIO_RECEBEDOR
  └──▶ REMOVIDA_PELO_PSP
```

Os três finais são **terminais**. Transição fora deste desenho é recusada no
domínio, antes de qualquer persistência. `CONCLUIDA` só é alcançada por
evidência de pagamento: webhook do banco ou conciliação (RN-31) — nunca por
chamada da API do cliente.

### RN-13 · Cobrança imediata (`cob`)

- Expiração padrão: **86400 s** (24 h, o default da spec), configurável por cobrança.
- Valor pode ser fixo ou permitir alteração pelo pagador (`modalidadeAlteracao`).
- Cobrança expirada continua `ATIVA` na spec do BCB; o Vintém expõe o campo
  derivado `expirada: true` (extensão) para não obrigar o cliente a calcular.

### RN-14 · Cobrança com vencimento (`cobv`)

- Exige devedor identificado: CPF ou CNPJ válido (inclusive o CNPJ alfanumérico,
  `^[0-9A-Z]{14}$`) e nome. UF, se vier, são 2 letras maiúsculas; CEP, 8 dígitos.
- `dataDeVencimento` é uma **data civil** avaliada no fuso de **Brasília**; não pode
  estar no passado. `validadeAposVencimento` (dias corridos, padrão **30**) define o
  último dia para pagar; depois dele a cobv aparece com `expirada`.
- Usa a mesma máquina de estados da cob (RN-12) e compartilha o **txid** com ela:
  um txid usado numa cob não serve para uma cobv, e vice-versa (garantido por
  gatilho no banco).
- O Vintém **valida** as combinações; o **cálculo final é do banco**, e o valor que
  vale é o que o banco liquidou:

| Campo | Modalidades | Regras |
|---|---|---|
| `multa` | 1 valor fixo · 2 percentual | valor positivo; percentual até 100,00 |
| `juros` | 1 a 8 (valor ou percentual, ao dia/mês/ano, dias corridos ou úteis) | valor positivo |
| `abatimento` | 1 valor fixo · 2 percentual | fixo menor que o valor original; percentual até 100,00 |
| `desconto` | 1 e 2: até datas fixas · 3 a 6: por antecipação | 1–2 usam `descontoDataFixa` (1 a 3 datas, sem repetir, até o vencimento) e nunca `valorPerc`; 3–6 usam `valorPerc` e nunca datas; fixo menor que o valor; percentual menor que 100,00 |

- No `PATCH`, cada bloco (`multa`, `juros`, `abatimento`, `desconto`) que vier
  **substitui** o anterior inteiro, e a cobv resultante é revalidada por completo
  (um abatimento que ficou maior que o valor revisado é recusado).

### RN-15 · BR Code

- Gerado em conformidade com o padrão EMV MPM do Manual de BR Code do BCB, com
  CRC16-CCITT (`0xFFFF`, polinômio `0x1021`).
- Dinâmico (URL de location do banco) para `cob`/`cobv`; estático para uso
  avulso via `pkg/brcode`.
- O Vintém **nunca** gera BR Code dinâmico com location própria: a location é
  sempre a do PSP da conta.

---

## Devoluções

### RN-20 · Estados da devolução

```
EM_PROCESSAMENTO ──▶ DEVOLVIDO
                 └─▶ NAO_REALIZADO
```

- O banco é a fonte da verdade sobre o resultado. Ele chega pelas `devolucoes`
  do aviso de Pix (`POST {webhookUrl}/pix`, RN-30) ou pela conciliação (RN-31),
  que consulta `GET /pix/{e2eid}/devolucao/{id}` de cada devolução ainda
  `EM_PROCESSAMENTO`. Cada mudança grava status, motivo, `rtrId` e liquidação e
  publica `devolucao.atualizada`. Aviso repetido não publica de novo.
- Devolução que o Vintém não pediu, de outra conta ou com status que volta
  atrás é ignorada com aviso no log, sem erro: responder erro faria o banco
  reenviar para sempre.

### RN-21 · Limites

- A soma das devoluções de um Pix (`endToEndId`) não ultrapassa o valor recebido.
  Devolução `NAO_REALIZADO` não conta: o valor dela volta a estar disponível.
- Devolução identificada pelo par `e2eid` + `id` (id definido pelo cliente, de 1
  a 35 caracteres `[a-zA-Z0-9]`). **Idempotente:** repetir o par com o mesmo
  valor devolve a devolução existente sem chamar o banco; com outro valor, 409.
- Prazo máximo de **90 dias** após o pagamento — validado antes de chamar o banco.
- Rotas da spec: `PUT /pix/{e2eid}/devolucao/{id}` (escopo `devolucao:write`) e
  `GET /pix/{e2eid}/devolucao/{id}` (escopo `pix:read`).
- Natureza `ORIGINAL` (o default). `RETIRADA` é a devolução de Pix Saque ou
  Troco, que o Vintém não recebe, e é recusada. Descrição até 140 caracteres.
- Tudo isso é conferido **antes de chamar o banco**. Devoluções simultâneas do
  mesmo Pix são feitas uma de cada vez, para que juntas não passem do valor.
- O Pix precisa estar gravado no Vintém (webhook ou conciliação) e ser da conta
  da API key restrita; senão, 404. Conta desativada continua devolvendo: o Pix
  já entrou.
- Cada devolução aceita pelo banco é gravada com o evento `devolucao.solicitada`.
  Se o banco falhar, nada é gravado nem publicado.

---

## Pix Automático

> ⚠️ **Pendente do estudo #1**: quais bancos expõem `/rec` e `/cobr` em
> produção para PJ. As regras abaixo descrevem o contrato da spec; a matriz de
> capacidades (RN-50) diz quem suporta.

### RN-40 · Recorrência (`/rec`)

```
CRIADA ──▶ APROVADA ──▶ CANCELADA
   │          └───────▶ EXPIRADA
   ├──▶ REJEITADA
   ├──▶ EXPIRADA
   └──▶ CANCELADA
```

Os diagramas da RN-40 a RN-42 seguem a seção 3 (Estados das entidades) do Pix
Automático no *Manual de Padrões para Iniciação do Pix* 2.10.0 do BCB,
conferida em 29/10/2026 (#99). `REJEITADA` só acontece na jornada 1; nas
jornadas com QR Code não há aviso de negativa.

- Os status são os da API Pix do BCB (`RecStatus`, 2.10.0): `CRIADA`, `APROVADA`,
  `REJEITADA`, `EXPIRADA`, `CANCELADA`.
- Uma recorrência tem periodicidade (semanal, mensal, trimestral, semestral ou
  anual), valor, vigência e política de retentativa
  (`NAO_PERMITE` ou `PERMITE_3R_7D`), definida só pelo recebedor (RN-42).
- **Valor:** fixo (`valorRec`, igual em todo ciclo) ou variável. No variável, o
  **pagador** define um valor máximo na autorização, e o recebedor pode exigir
  um **piso para esse máximo** (`valorMinimoRecebedor`): com piso de R$ 50,00,
  o pagador não autoriza um máximo de R$ 30,00. Fixo e piso são excludentes.
- Só gera cobrança (`cobr`) quando `APROVADA`.
- **Pela API, a recorrência é criada com uma API key restrita a uma conta.** É
  ela que identifica o recebedor, como a credencial do cliente na spec do BCB;
  key irrestrita responde 400. Consultar e pedir confirmação seguem a RN-60:
  recorrência de outra conta responde 404.
- Rejeição pelo pagador vem com motivo; o Vintém guarda o motivo (código e
  descrição do encerramento).
- **Mudanças chegam pelo webhook de recorrência** (`/webhooks/rec/{conta}/{token}`,
  com ou sem o `/rec` que a spec manda acrescentar), registrado por
  `vintem conta webhook` com o mesmo token do aviso de Pix. Cada mudança grava a
  recorrência, encerra as solicitações pendentes dela (RN-41) e publica
  `recorrencia.atualizada`, numa transação só. Aviso repetido não muda nada;
  aviso fora da RN-40 (uma volta de `APROVADA` para `CRIADA`, por exemplo) é
  ignorado com registro no log; recorrência desconhecida é ignorada, para o banco
  não reenviar para sempre.
- **Aviso perdido não trava a recorrência:** a conciliação (RN-31) também consulta
  no banco as recorrências abertas (`CRIADA` e `APROVADA`) e aplica o que mudou
  pelo mesmo caminho do webhook.
- A autorização é dada por uma de quatro **jornadas** do BCB: (1) notificação no
  app do pagador, sem QR Code (`/solicrec`); (2) QR Code só com a recorrência;
  (3) QR Code com primeiro pagamento imediato e recorrência; (4) QR Code de fatura
  com a recorrência. O PSP do recebedor oferece ao menos uma; o adapter declara
  quais (RN-50).

### RN-41 · Solicitação de confirmação (`/solicrec`)

É o pedido de autorização enviado ao app do pagador (jornada 1). Tem estados
próprios, separados dos da recorrência (`SolicRecStatus`, 2.10.0):

```
CRIADA ──▶ ENVIADA ──▶ RECEBIDA ──▶ ACEITA
   │          ├───────────┼────────▶ REJEITADA
   ├──────────┼───────────┼────────▶ EXPIRADA
   └──────────┴───────────┴────────▶ CANCELADA
```

- `CRIADA` não vai direto a `REJEITADA`. Em `ENVIADA` o PSP do pagador já pode
  rejeitar (conta inexistente, por exemplo); em `RECEBIDA`, o usuário pagador.
- Cancela quando a recorrência é cancelada ou aprovada por outra jornada, quando
  o recebedor desiste antes da resposta, ou por timeout: sem a confirmação de
  recebimento (pain.012) em 1 minuto, o PSP do recebedor cancela.
- `ACEITA` leva a recorrência a `APROVADA`; `REJEITADA`, a `REJEITADA` (RN-40).
- Fica pendente no app do pagador por **no máximo 30 dias**; o recebedor pode
  definir menos. Passado o prazo, expira.
- O recebedor só cancela (`PATCH /solicrec/{idSolicRec}` com `CANCELADA`), enquanto
  o pagador não respondeu. Outro status no corpo é recusado. A recorrência continua
  `CRIADA`, e o cancelamento publica `recorrencia.solicitacao` com o status novo.
- Mudanças chegam por webhook de recorrência (`/webhookrec`).

### RN-42 · Cobrança recorrente (`/cobr`)

- Status da API Pix (`CobRStatus`, 2.10.0): `CRIADA`, `ATIVA`, `CONCLUIDA`,
  `EXPIRADA`, `REJEITADA`, `CANCELADA`. O recebedor só cancela: outro status no
  `PATCH` é recusado.

  ```
  CRIADA ──▶ ATIVA ──▶ CONCLUIDA
     │         ├─────▶ EXPIRADA
     │         ├─────▶ REJEITADA
     └─────────┴─────▶ CANCELADA
  ```

  `CRIADA` é a cobrança registrada no PSP do recebedor; `ATIVA`, a enviada ao
  PSP do pagador para agendamento. A partir daí o andamento fica nas
  tentativas (`SOLICITADA → AGENDADA → PAGA`, ou `REJEITADA`, `CANCELADA`,
  `EXPIRADA`), e a cobrança **continua `ATIVA` durante as retentativas**. A
  rejeição pelo PSP do pagador passa por `ATIVA`; só alguns códigos da pain.014
  rejeitam a cobrança (os listados em `encerramento.rejeicao.codigo`), os
  outros rejeitam só a tentativa.
- **Só nasce de recorrência `APROVADA`**, com o vencimento dentro da vigência
  dela e, se a recorrência tem valor fixo, **com esse valor**. A cobrança é da
  conta da recorrência; key restrita a outra conta recebe 404 (RN-60).
- **Uma cobrança por ciclo:** enquanto a do ciclo está `CRIADA`, `ATIVA` ou
  `CONCLUIDA`, outra no mesmo ciclo é recusada. Depois de cancelada, rejeitada ou
  expirada, o recebedor pode criar outra.
- Pedir retentativa publica `cobranca_recorrente.tentativa`, com a data nova;
  cancelar publica `cobranca_recorrente.atualizada`. Se o aviso do banco chegou
  antes da resposta do PSP, nada é publicado de novo.
- **Antecedência:** a instrução de pagamento chega ao PSP do pagador **de 2 a 10
  dias corridos** antes da data prevista de liquidação. O Vintém recusa enviar
  fora dessa janela, em vez de deixar o banco recusar.
- **Retentativa** (`POST /cobr/{txid}/retentativa/{data}`), só se a recorrência
  for `PERMITE_3R_7D`: no máximo **3 datas** diferentes, em até **7 dias corridos**
  da data original, sem passar do início do ciclo seguinte (ou do fim da
  recorrência), com **o mesmo valor**, pedida até **23h59 do dia anterior** à nova data.
  Os dias são contados no fuso de Brasília. Os ciclos contam a partir da
  `dataInicial`; nos mensais, o dia que não existe no mês vira o último dia
  dele (31/01 → 28/02 → 31/03).
- **Mudanças chegam pelo webhook de cobrança recorrente**
  (`/webhooks/cobr/{conta}/{token}`, com ou sem o `/cobr` que a spec manda
  acrescentar), separado do de recorrência e registrado por `vintem conta
  webhook` com o mesmo token. Cada aviso grava a cobrança e publica, numa
  transação só, `cobranca_recorrente.tentativa` para cada tentativa nova ou que
  mudou e `cobranca_recorrente.atualizada` se o status mudou. O banco é a fonte
  da verdade sobre cada tentativa (identificada pela data e pelo tipo); a
  retentativa que só o Vintém conhece continua até o banco avisar. Aviso
  repetido não muda nada. O status do banco vale se for **alcançável** pelo
  diagrama, mesmo pulando passos: se o aviso de `ATIVA` se perdeu, `CONCLUIDA`
  vale direto de `CRIADA`. Aviso que volta no diagrama (de `CONCLUIDA` para
  `ATIVA`, por exemplo) é ignorado com registro no log; cobrança desconhecida é
  ignorada, para o banco não reenviar para sempre.
- **Sem aviso, a consulta e a conciliação acham a mudança:** `GET /cobr/{txid}`
  pergunta ao PSP enquanto a cobrança está aberta (se o PSP falhar, devolve a
  gravada), e `vintem conciliar` confere todas as abertas de cada conta pelo
  mesmo caminho do webhook.

### RN-43 · Prazos versionados

Antecedência, janelas de retentativa e afins **não são constantes no código**.
Ficam numa tabela de parâmetros com **data do normativo** de origem, para que
cada mudança do BCB vire um commit pequeno, revisável e com fonte — e um
release com nota. A tabela fica em `internal/domain/parametros.go`: cada linha
tem a fonte e a data a partir da qual vale, e uma linha que já vale nunca é
editada; mudança entra como linha nova.

Valores de partida, do *Guia de implementação do Pix Automático* v1.3
(BCB, 19/08/2026), conferidos pelo estudo #1:

| Parâmetro | Valor | Onde |
|---|---|---|
| Pendência máxima da solicitação de confirmação | 30 dias | Guia, seção 3.2 |
| Antecedência da instrução de pagamento | 2 a 10 dias corridos | Guia, agendamento |
| Retentativas pós-vencimento | até 3 datas, em até 7 dias corridos, mesmo valor | Guia, novas tentativas |
| Prazo para pedir a retentativa | até 23h59 do dia anterior | Guia, novas tentativas |

---

## Webhooks

### RN-30 · Entrada (banco → Vintém)

- A URL de aviso é **por conta**, com um token secreto:
  `https://<seu-vintem>/webhooks/pix/{conta}/{token}`. Ela é gerada por
  `vintem conta webhook <id> --url-publica https://...`, que também a registra no
  PSP. Só o hash do token é gravado; rodar de novo troca o token (e o antigo para
  de valer). O token faz parte da URL: trate a URL como segredo. Ele é gravado
  **antes** de falar com o banco, porque o Efí chama a URL durante o registro; se o
  banco recusar, o token anterior volta a valer.
- A spec do BCB manda o banco acrescentar `/pix` à URL: os dois caminhos são aceitos.
- A rota não exige API key (quem chama é o banco). Token ausente, errado ou fora
  do formato responde **401**, sem dizer qual dos três.
- O corpo é interpretado pelo **adapter da conta**: quem segue a spec usa o
  formato padrão (`{"pix": [...]}`), quem desvia trata o desvio no adapter. Um
  item inválido recusa o lote inteiro (**400**): melhor o banco reenviar que
  perder um Pix em silêncio.
- O evento é **persistido antes** de responder 200. Se a persistência falhar, o
  Vintém responde 5xx e deixa o banco reenviar.
- Deduplicação por `endToEndId`: o mesmo Pix avisado duas vezes gera um registro
  e um evento só.
- Pix com `txid` de uma cob ou cobv **ativa da mesma conta** conclui a cobrança
  na mesma transação. O banco é a fonte da verdade: valor diferente do cobrado
  ainda conclui, mas o evento `pix.recebido` sai com `divergente: true`. Pix sem
  txid, ou com txid de outra conta, é registrado sem concluir nada.
- Corpo acima de 1 MiB é recusado em qualquer rota.
- Os Pix gravados saem pelas rotas da spec: `GET /pix` (janela `[inicio, fim)`
  pelo horário do Pix, com os filtros `txid`, `txIdPresente`, `devolucaoPresente`,
  `cpf` e `cnpj` do pagador, e a paginação da spec) e `GET /pix/{e2eid}`, os dois
  com escopo `pix:read` e com as devoluções de cada Pix. São os Pix que o Vintém
  conhece, não uma consulta ao banco: um Pix ainda não avisado nem conciliado não
  aparece. API key restrita a uma conta só vê os Pix dela; Pix de outra conta
  responde 404.

### RN-31 · Conciliação

Um job dentro do `serve` consulta `GET /pix` de **cada conta** numa janela
deslizante e registra todo Pix que não chegou por webhook, pelo mesmo caminho do
webhook (RN-30): deduplicado por `endToEndId`, concluindo a cobrança e publicando
`pix.recebido`. **Nenhum Pix some** porque o banco falhou em avisar.

Nas contas cujo banco faz Pix Automático, a mesma rodada consulta as
recorrências abertas (`CRIADA` e `APROVADA`) e aplica as mudanças sem aviso
pelo caminho do webhook de recorrência (RN-40). Uma recorrência que falha não
impede as outras, e as recorrências não dependem de a consulta de Pix dar certo.

A rodada também consulta no banco cada devolução `EM_PROCESSAMENTO` (RN-20).
Assim, o resultado da devolução de um Pix antigo, que já saiu da janela do
`GET /pix`, também chega mesmo sem aviso.

- Padrão: a cada **5 min**, olhando as últimas **2 h**
  (`VINTEM_CONCILIACAO_INTERVALO`, `VINTEM_CONCILIACAO_JANELA`). Intervalo `0`
  desliga, com aviso no log; janela zero é recusada.
- A janela é `[agora − janela, agora)`. Um Pix no instante exato de `agora` entra
  na rodada seguinte: as janelas se sobrepõem.
- Conta **desativada também entra**: as cobranças antigas dela ainda podem ser pagas.
- Uma conta que falha (PSP fora, PSP desconhecido) não impede as outras; a falha
  vai para o log e a rodada seguinte tenta de novo.
- As rodadas nunca se sobrepõem, e um atraso aleatório de até 10% evita que
  várias instâncias consultem o banco no mesmo segundo.
- `vintem conciliar` roda uma rodada sob demanda (útil depois de uma indisponibilidade).

### RN-32 · Saída (Vintém → aplicação do cliente)

- **Destinos:** `vintem destino criar --url https://... [--conta ID]`. Sem `--conta`,
  o destino recebe os eventos de todas as contas. O segredo `whsec_...` aparece
  **uma vez**, na criação, e fica cifrado pelo Vault (é preciso tê-lo em claro
  para assinar). https obrigatório (http só para localhost).
- **Eventos:** `cobranca.criada`, `cobranca.revisada`, `cobranca.concluida`,
  `cobranca.removida`, `pix.recebido` (com `divergente` quando o valor pago
  difere do cobrado), `devolucao.solicitada` (RN-21), `devolucao.atualizada` (RN-20), `certificado.expirando` (RN-62) e, do Pix Automático,
  `recorrencia.criada`, `recorrencia.atualizada` e `recorrencia.solicitacao`
  (RN-40, RN-41), e `cobranca_recorrente.criada`, `cobranca_recorrente.atualizada`
  e `cobranca_recorrente.tentativa` (RN-42). O corpo é o envelope do Standard Webhooks:
  `{"type": ..., "timestamp": ..., "data": {...}}`, normalizado, igual para
  qualquer banco.
- **Assinatura** no padrão [Standard Webhooks](https://www.standardwebhooks.com/):
  `webhook-id` (o id do evento), `webhook-timestamp` e `webhook-signature`
  (`v1,` + HMAC-SHA256). Verificável com as bibliotecas oficiais — o teste usa a
  de Go para conferir o que o Vintém envia.
- **Entrega *at-least-once*:** o cliente deve deduplicar pelo `webhook-id`, que é
  o mesmo em todas as tentativas. O corpo também é o mesmo, byte a byte.
- **Distribuição:** cada evento do outbox vira uma entrega por destino que casa
  com a conta. Evento sem destino na hora é descartado: destino criado depois não
  recebe o passado.
- **Retry:** sucesso é qualquer 2xx; redirecionamento não é seguido (conta como
  falha). Esperas de 5 s, 30 s, 2 min, 10 min, 30 min, 1 h, 2 h e depois 4 h, com
  até 10% de jitter, até **24 h** depois do evento; então a entrega vai para a
  **DLQ** (`vintem dlq listar`) e volta à fila com `vintem dlq reenviar <evento>`.
- Até 8 entregas em paralelo e 10 s de timeout por tentativa: um destino lento não
  trava os outros. O job roda a cada 2 s (`VINTEM_WEBHOOK_INTERVALO`; `0` desliga).
- Destino removido com entregas pendentes: elas vão para a DLQ.

---

## Segurança e acesso

### RN-60 · API keys

- Toda rota, exceto `/health`, exige `Authorization: Bearer vtm_<id>_<segredo>`.
- Escopos: `cob:read`, `cob:write`, `cobv:read`, `cobv:write`, `pix:read`,
  `devolucao:write`, `rec:read`, `rec:write`, `cobr:read`, `cobr:write`, `webhook:admin`, `conta:admin`.
  Cada rota exige exatamente um; rota sem escopo mapeado é **recusada** (falha
  fechado), e um teste aponta a rota esquecida.
- O segredo aparece **uma vez**, na criação. Só o hash SHA-256 é gravado: com
  190 bits de entropia, hash lento não acrescenta proteção.
- Chave restrita a uma conta só enxerga as cobranças dela. Cobrança de outra
  conta responde **404**, sem revelar que existe; chave de outra conta no corpo é
  tratada como chave desconhecida. Chave restrita não pode ter `conta:admin`.
- Revogação vale na hora. 401 não diz se a chave existe, está revogada ou
  malformada.
- Criação e revogação de chaves é pela CLI (`vintem apikey ...`), que é também
  como se cria a primeira chave de uma instância nova.

### RN-61 · Segredos em repouso

Certificados, chaves privadas e client secrets são cifrados com AES-256-GCM
usando `VINTEM_MASTER_KEY`. Sem a master key, o Vintém não sobe. Nada disso é
escrito em log — nem mascarado parcialmente.

- **Chave diferente é recusada ao abrir o banco:** todo segredo guardado é decifrado
  na abertura, e a falha diz qual conta ou destino não abriu. Sem isso, o erro só
  apareceria na primeira chamada ao banco.
- **Troca de chave:** `vintem chave trocar` recifra credenciais e segredos com
  `VINTEM_MASTER_KEY_NOVA` numa transação só, com o servidor parado.

### RN-62 · Certificados

- O certificado do mTLS é **conferido no cadastro** da conta, pela CLI ou pela API:
  .p12 com senha, ou PEM com certificado e chave no mesmo arquivo. O Vintém
  recusa, com o motivo e sem mostrar o conteúdo, arquivo ilegível, senha errada,
  chave privada que não corresponde ao certificado, chave PEM cifrada e
  certificado vencido ([ADR-006](./adr/006-certificado-conferido-no-cadastro.md)).
- Sujeito, impressão digital (SHA-256) e validade ficam visíveis na conta
  (`conta listar`, `GET /vintem/v1/contas`). São dados públicos do certificado;
  o certificado e a chave continuam cifrados (RN-61).
- **Aviso de vencimento a 30 dias:** o evento `certificado.expirando` sai pelo
  mesmo caminho dos outros webhooks de saída (RN-32), assinado e com retry,
  quando faltam 30 dias ou menos para o certificado de uma conta **ativa**
  vencer. Sai também para certificado que já venceu sem ter sido avisado, caso
  o Vintém tenha ficado desligado durante a janela.
  - **Uma vez por certificado:** o aviso é guardado pela impressão digital, na
    mesma transação do evento. Rodar de novo não repete; trocar o certificado
    zera o aviso.
  - Payload: `contaId`, `sujeito`, `impressaoDigital`, `validade`,
    `diasRestantes` (arredondado para cima; zero ou negativo depois de vencer)
    e `vencido`. Nunca leva o certificado.
  - A verificação roda ao subir e depois a cada hora
    (`VINTEM_CERTIFICADO_INTERVALO`; `0` desliga). Como todo evento, sem destino
    cadastrado na hora o aviso é descartado e não se repete.
  - O alerta no painel entra com o painel (F8).
  - **Contas cadastradas antes do v0.3.0**, quando o certificado não era lido
    no cadastro, ganham os dados públicos ao subir o `serve`. O certificado é
    lido do blob cifrado; vencido é gravado mesmo assim (e avisado), e ilegível
    gera um aviso no log com o id da conta e o motivo, sem nada do conteúdo.
- **Certificado de homologação não entra em conta de produção.** Nada no X.509
  diz o ambiente; para o Vintém, é de homologação o certificado (pela impressão
  digital) **já cadastrado numa conta de homologação**, mesmo que ela esteja
  desativada. A conta de produção é recusada com o nome da conta onde ele está.
  O contrário é aceito (certificado de produção numa conta de homologação), e
  várias contas do mesmo ambiente podem compartilhar o certificado. Motivo: o
  Bradesco, por exemplo, não aceita o mesmo certificado nos dois ambientes, e o
  erro só apareceria na primeira cobrança real.

---

## Adapters

### RN-50 · Matriz de capacidades

Cada adapter declara o que suporta: `cob`, `cobv`, `devolucao`, `webhook`,
`rec`, `cobr`, `lote`. Chamada a algo que o banco da conta não suporta responde
**501** com o nome da capacidade — nunca um erro genérico do banco.

### RN-51 · Conformidade

Adapter só entra no `develop` se passar na suíte de conformidade
(`test/conformance`) contra fixtures gravadas. Desvio de banco em relação à
spec é tratado **dentro** do adapter e documentado nele; o domínio nunca ganha
um `if banco == ...`.

### RN-52 · Sandbox

O adapter `sandbox` implementa todas as capacidades com estado em memória ou
SQLite: cobrança criada é paga após N segundos (configurável), pode simular
falha, expiração e webhook duplicado. É o adapter de referência da suíte.
