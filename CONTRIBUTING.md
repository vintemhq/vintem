# Como contribuir com o Vintém

Obrigado por querer ajudar! O jeito de maior impacto é **adicionar ou melhorar
o suporte a um banco**: cada banco novo é um público inteiro que passa a poder
usar o Vintém.

## Antes de começar

1. Leia o [manual do projeto](./docs/EQUIPE.md) (fluxo, DoR e DoD) e as [regras de negócio](./docs/REGRAS-DE-NEGOCIO.md).
2. Procure a issue no [board](https://github.com/orgs/vintemhq/projects). Não achou? Abra uma pelo template antes de escrever código — evita PR que não entra.
3. Comente na issue que vai pegar.

## Ambiente

```bash
git clone https://github.com/vintemhq/vintem && cd vintem
git checkout develop
make test   # go test -race ./...
make lint   # golangci-lint run
make run    # sobe em :8080
```

Requisitos: Go 1.27+, [golangci-lint](https://golangci-lint.run/) v2, Docker (para a imagem).

O `make test` inclui o `test/e2e`: ele compila o binário, sobe o `vintem sandbox`,
cria uma cobrança, espera o pagamento (5 s) e confere que os eventos chegam com
assinatura válida num servidor de teste. Para iterar rápido, `go test -short ./...`
pula essa parte.

## Fluxo

- Branch a partir de `develop`: `feature/descricao-curta` ou `fix/descricao-curta`.
- Commits em [Conventional Commits](https://www.conventionalcommits.org/), em português: `feat: adiciona adapter do Sicoob`.
- `Closes #N` na mensagem do commit.
- PR para `develop`. CI verde é obrigatório.

## Escrevendo um adapter de banco

1. Abra (ou pegue) a issue pelo template **🏦 Novo adapter de banco**.
2. Crie `internal/adapters/psp/<banco>/`. Banco que segue a API Pix do BCB não reescreve nada: o pacote `pixapi` já faz mTLS, OAuth2, cob, cobv, Pix recebidos, devolução e webhook. O adapter embute `*pixapi.API` e passa um `pixapi.Banco` com as URLs, a busca do token, a tradução dos erros e os desvios ligados (`ConsultarAntesDeCriar`, `HeadersWebhook`). O que for só do banco vira um método no pacote dele, que sobrescreve o do `pixapi`. O `efi` e o `inter` são os exemplos.
3. Declare em `Capabilities()` só o que o banco realmente suporta.
4. Trate **dentro do adapter** todo desvio do banco em relação à spec do BCB, com um comentário dizendo qual desvio e a fonte (link da doc do banco).
   - **Credenciais e mTLS:** o adapter recebe um `ports.Credenciais` no construtor e pede as credenciais da conta a cada chamada ([ADR-007](docs/adr/007-credenciais-chegam-ao-adapter-por-port.md)). O par do mTLS sai de `certificado.TLS` (`internal/platform/certificado`), que lê .p12 e PEM. O adapter `efi` é a referência.
   - **Token OAuth2:** não peça token a cada chamada. Escreva só a função que busca o token no banco (o formato muda de banco para banco; `oauth.LerResposta` lê a resposta padrão) e use o `oauth.Transport` de `internal/platform/oauth` no `http.Client`, com o transporte mTLS da conta como `Base`. Ele guarda o token por conta, faz uma só busca com chamadas concorrentes, renova antes de expirar e tenta de novo uma vez com token novo se o banco responder 401.
5. Rode a [suíte de conformidade](#suíte-de-conformidade) contra um servidor falso do banco e, quando tiver acesso à homologação, grave as fitas.
6. Atualize a tabela de bancos suportados no README.

Adapter só entra se passar na suíte de conformidade (RN-51).

## Suíte de conformidade

`test/conformance` é a régua única: os mesmos cenários rodam contra todo
adapter, para que nenhum deles interprete a spec do BCB do seu jeito e o domínio
nunca precise de um `if banco == ...`. Os cenários são agrupados por
capacidade; o que o adapter não declara em `Capabilities()` é pulado, e um
cenário próprio confere que o não declarado responde `ports.ErrNaoSuportado`.

O adapter roda a suíte a partir do próprio pacote de teste:

```go
func TestConformidade(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		// Um adapter novo a cada cenário, apontando para o servidor falso do banco.
		return conformance.Alvo{PSP: p, Conta: conta, Agora: relogio.Now, Pagar: pagar}
	})
}
```

- **`Pagar`** faz uma cobrança ser paga. Só dá para fazer contra o sandbox ou o servidor falso; com `nil`, os cenários de pagamento e devolução são pulados.
- **Servidor falso:** escreva um em `internal/adapters/psp/<banco>/<banco>test`, seguindo os exemplos da documentação do banco (veja `efi/efitest`). É ele que roda no CI a cada PR.
- **Sandbox:** precisa passar a suíte inteira, sem nada pulado.

### Fitas da homologação

O servidor falso mostra o que *você* entendeu da documentação. A fita mostra o que
o banco *de fato* responde. O pacote `test/conformance/fita` grava as chamadas
HTTP feitas contra a homologação e depois as reproduz no CI, sem rede e sem
credencial:

```go
f := fita.Abrir(t, fita.Arquivo("../../../../test/fixtures/<banco>", t.Name()), gravar)
p := banco.New(cred, fake.NewClock(f.Agora), banco.ComTransporte(f.Envolver))
return conformance.Alvo{PSP: p, Conta: conta, Agora: relogio.Now, Rodada: f.Rodada}
```

- O adapter precisa de uma opção que envolva o transporte mTLS, token incluído (`efi.ComTransporte`).
- Na reprodução, o adapter tem que fazer as mesmas chamadas, na mesma ordem e com os mesmos corpos JSON. Se ele mudar o que envia, grave de novo.
- `Rodada` muda a cada gravação, porque o banco lembra os txids da anterior.
- A fita não guarda cabeçalhos, troca o `access_token` por `"gravado"` e troca `recebedor` e `pagador` (nome, CPF/CNPJ, endereço) por dados fictícios. Mesmo assim, **abra os JSON antes do commit** e confira se não sobrou dado real (use uma chave Pix aleatória). Se o teste falhar gravando, a fita fica num `.falha` ignorado pelo git, só para diagnóstico.
- Se o banco testar a URL do webhook antes de aceitar (o Efí testa), passe uma URL pública em `URLWebhook`; sem ela, o cenário é pulado.
- O `TestConformidadeGravada` do Efí é o modelo, com o comando de gravação no comentário.

## O que não entra

Para manter o projeto sustentável, o escopo é fechado: cobrança, devolução, Pix
Automático, webhooks, conciliação, painel e DX. Boleto, cartão, split,
antecipação e emissão de nota ficam de fora — abra uma Discussion se quiser
argumentar.

## Segurança

Nunca abra issue pública para vulnerabilidade. Veja o [SECURITY.md](./SECURITY.md).
