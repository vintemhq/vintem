// Package cli implementa os comandos do binário vintem.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/httpapi"
	"github.com/vintemhq/vintem/internal/adapters/jobs"
	"github.com/vintemhq/vintem/internal/adapters/notifier"
	"github.com/vintemhq/vintem/internal/adapters/store/sqlite"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/vault"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/web"
)

// Fontes é o que os adapters de PSP recebem ao serem montados.
type Fontes struct {
	// Credenciais decifra as credenciais de uma conta (ADR-007).
	Credenciais ports.Credenciais
	// Store é o store do Vintém; o sandbox o lê para lembrar das cobranças depois de reiniciar.
	Store ports.Store
}

// Ambiente é o que a CLI recebe do processo; nos testes, tudo é falso.
type Ambiente struct {
	Saida  io.Writer
	Getenv func(string) string
	// PSPs é o registry usado quando MontarPSPs é nil (os testes).
	PSPs ports.Registry
	// MontarPSPs monta o registry com o que só existe depois de abrir o store e
	// o vault: as credenciais das contas (bancos reais) e o próprio store (sandbox).
	MontarPSPs func(Fontes) ports.Registry
	Clock      ports.Clock
	Versao     string
}

const uso = `uso: vintem <comando>

  serve                          sobe a API (padrão)
  sandbox                        sobe a API pronta para testar, sem banco nem configuração
  mcp                            servidor MCP para agentes de IA (stdio), sobre a API
                                 em VINTEM_URL com a API key de VINTEM_TOKEN
  version                        imprime a versão
  chave gerar                    gera uma VINTEM_MASTER_KEY nova
  chave trocar                   recifra tudo com a chave de VINTEM_MASTER_KEY_NOVA
                                 (pare o serve antes)
  conta criar  [flags]           cadastra uma conta recebedora
  conta listar                   lista as contas
  conta desativar <id>           impede novas cobranças na conta
  conta webhook <id> --url-publica https://...
                                 gera a URL de aviso de Pix e registra no banco
  conta webhook <id> --remover   cancela o aviso no banco e invalida a URL
  conciliar                      roda a conciliação uma vez (GET /pix de cada conta)
  destino criar --url URL [--conta ID]
                                 cadastra onde entregar os eventos (sem --conta: todas)
  destino listar                 lista os destinos
  destino remover <id>           remove um destino
  dlq listar                     lista as entregas que desistiram depois de 24 h
  dlq reenviar <evento-id>       devolve as entregas do evento à fila
  apikey criar --nome N --escopo E [--escopo E...] [--conta ID]
                                 cria uma API key (o token aparece uma vez só)
  apikey listar                  lista as API keys
  apikey revogar <id>            revoga uma API key na hora

escopos: cob:read cob:write cobv:read cobv:write pix:read devolucao:write
         rec:read rec:write cobr:read cobr:write webhook:admin conta:admin

variáveis: VINTEM_DB (padrão vintem.db), VINTEM_MASTER_KEY, VINTEM_ADDR (padrão :8080),
           VINTEM_CONCILIACAO_INTERVALO (padrão 5m; 0 desliga), VINTEM_CONCILIACAO_JANELA (padrão 2h),
           VINTEM_WEBHOOK_INTERVALO (padrão 2s; 0 desliga a entrega de eventos),
           VINTEM_CERTIFICADO_INTERVALO (padrão 1h; 0 desliga o aviso de certificado vencendo),
           VINTEM_MASTER_KEY_NOVA (só para chave trocar)`

// Run executa o comando em args.
func Run(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	if len(args) > 1 && slices.ContainsFunc(args[1:], pedeAjuda) {
		return ajuda(ctx, args, amb)
	}
	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(amb.Saida, amb.Versao)
		return err
	case "serve":
		return serve(ctx, amb)
	case "sandbox":
		return sandboxCmd(ctx, amb)
	case "mcp":
		return mcpCmd(ctx, amb)
	case "chave":
		if len(args) >= 2 && args[1] == "trocar" {
			return chaveTrocar(ctx, amb)
		}
		if len(args) < 2 || args[1] != "gerar" {
			return errors.New("use: vintem chave gerar|trocar")
		}
		_, err := fmt.Fprintln(amb.Saida, vault.NovaChave())
		return err
	case "conta":
		return conta(ctx, args[1:], amb)
	case "apikey":
		return apikey(ctx, args[1:], amb)
	case "conciliar":
		return conciliar(ctx, amb)
	case "destino":
		return destino(ctx, args[1:], amb)
	case "dlq":
		return dlq(ctx, args[1:], amb)
	case "help", "-h", "--help":
		_, err := fmt.Fprintln(amb.Saida, uso)
		return err
	default:
		return fmt.Errorf("comando desconhecido %q\n\n%s", args[0], uso)
	}
}

func pedeAjuda(arg string) bool { return arg == "-h" || arg == "--help" }

// ajuda mostra o uso de um comando sem rodá-lo: nada de subir o sandbox nem
// pedir a VINTEM_MASTER_KEY só para ler a ajuda. Mostra as linhas do uso que
// começam pelo comando e, no conta criar, as flags.
func ajuda(ctx context.Context, args []string, amb Ambiente) error {
	prefixo := "  " + args[0]
	if len(args) > 2 && !pedeAjuda(args[1]) && !strings.HasPrefix(args[1], "-") {
		prefixo += " " + args[1]
	}
	var linhas []string
	continua := false
	for _, l := range strings.Split(uso, "\n") {
		switch {
		case strings.HasPrefix(l, prefixo+" ") || l == prefixo:
			linhas, continua = append(linhas, l), true
		case continua && strings.HasPrefix(l, "      "):
			linhas = append(linhas, l)
		default:
			continua = false
		}
	}
	if len(linhas) == 0 {
		_, err := fmt.Fprintln(amb.Saida, uso)
		return err
	}
	if _, err := fmt.Fprintln(amb.Saida, "uso: vintem <comando>\n\n"+strings.Join(linhas, "\n")); err != nil {
		return err
	}
	if prefixo == "  conta criar" {
		// As flags vêm do próprio FlagSet: o -h faz o Parse imprimir e parar
		// antes de usar as contas.
		_, _ = fmt.Fprintln(amb.Saida)
		if err := contaCriar(ctx, app.Contas{}, []string{"-h"}, amb); !errors.Is(err, flag.ErrHelp) {
			return err
		}
	}
	return nil
}

func getenv(amb Ambiente, chave, padrao string) string {
	if v := amb.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

// abrir monta o caso de uso de contas a partir do ambiente.
func abrir(ctx context.Context, amb Ambiente) (app.Contas, func(), error) {
	chave := amb.Getenv("VINTEM_MASTER_KEY")
	if chave == "" {
		return app.Contas{}, nil, errors.New("defina VINTEM_MASTER_KEY (gere uma com `vintem chave gerar` e guarde em lugar seguro: sem ela as credenciais não são recuperáveis)")
	}
	v, err := vault.New(chave)
	if err != nil {
		return app.Contas{}, nil, err
	}
	if err := recusarBancoDoLastro(amb); err != nil {
		return app.Contas{}, nil, err
	}
	store, err := sqlite.Open(ctx, getenv(amb, "VINTEM_DB", "vintem.db"))
	if err != nil {
		return app.Contas{}, nil, err
	}
	contas := app.Contas{Store: store, Vault: v, PSPs: amb.PSPs, Certificados: leitorCertificados{}, Clock: amb.Clock}
	if err := contas.ConferirChaveMestra(ctx); err != nil {
		_ = store.Close()
		return app.Contas{}, nil, fmt.Errorf("VINTEM_MASTER_KEY não abre os dados de %s (%w): use a chave com que eles foram gravados", getenv(amb, "VINTEM_DB", "vintem.db"), err)
	}
	if amb.MontarPSPs != nil {
		// Contas.Credenciais só usa Store e Vault, já prontos aqui.
		contas.PSPs = amb.MontarPSPs(Fontes{Credenciais: contas, Store: store})
	}
	return contas, func() { _ = store.Close() }, nil
}

// recusarBancoDoLastro evita o engano de quem atualizou do Lastro sem renomear o
// banco: o Vintém criaria um vintem.db vazio e pareceria que os dados sumiram.
func recusarBancoDoLastro(amb Ambiente) error {
	if amb.Getenv("VINTEM_DB") != "" {
		return nil
	}
	if _, err := os.Stat("vintem.db"); err == nil {
		return nil
	}
	if _, err := os.Stat("lastro.db"); err != nil {
		return nil
	}
	return errors.New("encontrei lastro.db e nenhum vintem.db: pare o processo e renomeie o arquivo (com o -wal e o -shm, se existirem) ou defina VINTEM_DB=lastro.db")
}

// chaveTrocar recifra as credenciais e os segredos com VINTEM_MASTER_KEY_NOVA.
func chaveTrocar(ctx context.Context, amb Ambiente) error {
	textoNova := amb.Getenv("VINTEM_MASTER_KEY_NOVA")
	if textoNova == "" {
		return errors.New("defina VINTEM_MASTER_KEY_NOVA (gere uma com `vintem chave gerar`)")
	}
	if textoNova == amb.Getenv("VINTEM_MASTER_KEY") {
		return errors.New("VINTEM_MASTER_KEY_NOVA é igual à VINTEM_MASTER_KEY: gere outra com `vintem chave gerar`")
	}
	nova, err := vault.New(textoNova)
	if err != nil {
		return fmt.Errorf("VINTEM_MASTER_KEY_NOVA: %w", err)
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	nContas, nDestinos, err := contas.TrocarChaveMestra(ctx, nova)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(amb.Saida, "%d contas e %d destinos recifrados.\nAgora a chave é a nova: troque VINTEM_MASTER_KEY e suba o Vintém.\n", nContas, nDestinos)
	return err
}

func conta(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: vintem conta criar|listar|desativar")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()

	switch args[0] {
	case "criar":
		return contaCriar(ctx, contas, args[1:], amb)
	case "listar":
		lista, err := contas.Listar(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNOME\tPSP\tCHAVE\tAMBIENTE\tATIVA\tCERTIFICADO VALE ATÉ")
		for _, c := range lista {
			validade := "-"
			if c.Certificado != nil {
				validade = c.Certificado.Validade.Format(time.DateOnly)
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%v\t%s\n", c.ID, c.Nome, c.PSP, c.ChavePix, c.Ambiente, c.Ativa, validade)
		}
		return tw.Flush()
	case "webhook":
		fs := flag.NewFlagSet("conta webhook", flag.ContinueOnError)
		fs.SetOutput(amb.Saida)
		urlPublica := fs.String("url-publica", "", "endereço público deste Vintém (https://vintem.suaempresa.com)")
		remover := fs.Bool("remover", false, "cancela o aviso registrado no banco e invalida a URL")
		if len(args) < 2 || strings.HasPrefix(args[1], "-") {
			return errors.New("use: vintem conta webhook <id> --url-publica URL (ou --remover)")
		}
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if *remover && *urlPublica != "" {
			return errors.New("use --url-publica para registrar ou --remover para cancelar, não os dois")
		}
		wh := app.Webhooks{Store: contas.Store, PSPs: contas.PSPs, Recebimentos: app.Recebimentos{Store: contas.Store, Clock: amb.Clock},
			Recorrencias:         app.Recorrencias{Store: contas.Store, PSPs: contas.PSPs, Clock: amb.Clock},
			CobrancasRecorrentes: app.CobrancasRecorrentes{Store: contas.Store, PSPs: contas.PSPs, Clock: amb.Clock}}
		if *remover {
			registrado, err := wh.Remover(ctx, args[1])
			if err != nil {
				return err
			}
			msg := "webhook da conta %s cancelado no PSP e URL invalidada: avisos para ela agora respondem 401.\n"
			if !registrado {
				msg = "o PSP não tinha webhook registrado na chave da conta %s; a URL local foi invalidada.\n"
			}
			_, err = fmt.Fprintf(amb.Saida, msg, args[1])
			return err
		}
		endereco, err := wh.Configurar(ctx, args[1], *urlPublica)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(amb.Saida, "webhook da conta %s registrado no PSP:\n\n  %s\n\nO token faz parte da URL: trate a URL como segredo. Rodar de novo troca o token.\n", args[1], endereco)
		return err
	case "desativar":
		if len(args) != 2 {
			return errors.New("use: vintem conta desativar <id>")
		}
		if err := contas.Desativar(ctx, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(amb.Saida, "conta %s desativada: novas cobranças serão recusadas\n", args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use criar, listar ou desativar)", args[0])
	}
}

func contaCriar(ctx context.Context, contas app.Contas, args []string, amb Ambiente) error {
	fs := flag.NewFlagSet("conta criar", flag.ContinueOnError)
	fs.SetOutput(amb.Saida)
	var c domain.Conta
	var ambiente, secretArq, certArq, senhaArq string
	var cred domain.Credenciais
	fs.StringVar(&c.ID, "id", "", "identificador da conta ([a-z0-9-])")
	fs.StringVar(&c.Nome, "nome", "", "nome para exibição")
	fs.StringVar(&c.PSP, "psp", "", "adapter do banco (sandbox, efi, inter, sicoob, bb)")
	fs.StringVar(&c.ChavePix, "chave", "", "chave Pix que recebe as cobranças")
	fs.StringVar(&ambiente, "ambiente", string(domain.Homologacao), "homologacao ou producao")
	fs.StringVar(&cred.ClientID, "client-id", "", "client id do OAuth2 do banco")
	fs.StringVar(&cred.ChaveAplicacao, "chave-aplicacao", "", "chave de aplicação do portal do BB (gw-dev-app-key); só para o bb")
	// Segredos vêm de arquivo, nunca de flag: flag fica no histórico do shell.
	fs.StringVar(&secretArq, "client-secret-arquivo", "", "arquivo com o client secret")
	fs.StringVar(&certArq, "certificado", "", "arquivo do certificado do mTLS (.p12 ou .pem)")
	fs.StringVar(&senhaArq, "senha-certificado-arquivo", "", "arquivo com a senha do certificado")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.Ambiente = domain.Ambiente(ambiente)

	var err error
	if cred.ClientSecret, err = lerSegredo(secretArq); err != nil {
		return err
	}
	if cred.SenhaCertificado, err = lerSegredo(senhaArq); err != nil {
		return err
	}
	if certArq != "" {
		if cred.Certificado, err = os.ReadFile(certArq); err != nil { //nolint:gosec // G304: caminho informado pelo próprio operador
			return fmt.Errorf("lendo certificado: %w", err)
		}
	}
	if err := contas.Criar(ctx, c, cred); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(amb.Saida, "conta %s criada (%s, %s)\n", c.ID, c.PSP, c.Ambiente); err != nil {
		return err
	}
	if criada, err := contas.Buscar(ctx, c.ID); err == nil && criada.Certificado != nil {
		cert := criada.Certificado
		_, err = fmt.Fprintf(amb.Saida, "certificado %s, válido até %s (SHA-256 %s)\n", cert.Sujeito, cert.Validade.Format(time.DateOnly), cert.ImpressaoDigital)
		return err
	}
	return nil
}

func lerSegredo(arquivo string) (string, error) {
	if arquivo == "" {
		return "", nil
	}
	b, err := os.ReadFile(arquivo) //nolint:gosec // G304: caminho informado pelo próprio operador
	if err != nil {
		return "", fmt.Errorf("lendo %s: %w", arquivo, err)
	}
	return string(trimFinal(b)), nil
}

// trimFinal tira a quebra de linha que editores e `echo` deixam no fim do arquivo.
func trimFinal(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func serve(ctx context.Context, amb Ambiente) error {
	addr := getenv(amb, "VINTEM_ADDR", ":8080")
	intervalo, err := duracao(amb, "VINTEM_CONCILIACAO_INTERVALO", "5m")
	if err != nil {
		return err
	}
	conc, err := conciliacao(amb)
	if err != nil {
		return err
	}
	intervaloWebhook, err := duracao(amb, "VINTEM_WEBHOOK_INTERVALO", "2s")
	if err != nil {
		return err
	}
	intervaloCertificado, err := duracao(amb, "VINTEM_CERTIFICADO_INTERVALO", "1h")
	if err != nil {
		return err
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	// Os jobs param e terminam antes de o store fechar: o defer abaixo roda antes
	// do fechar() acima, inclusive quando o servidor nem chega a subir.
	// Contas de antes do v0.3.0 ganham os dados públicos do certificado antes de
	// os jobs rodarem, para o aviso de vencimento já as enxergar (#79).
	if n, err := contas.PreencherCertificados(ctx); err != nil {
		return err
	} else if n > 0 {
		slog.InfoContext(ctx, "dados do certificado preenchidos em contas antigas", "contas", n)
	}
	ctxJobs, pararJobs := context.WithCancel(ctx)
	var rodando sync.WaitGroup
	defer func() {
		pararJobs()
		rodando.Wait()
	}()
	store := contas.Store
	conc.Store, conc.Recebimentos.Store, conc.PSPs = store, store, contas.PSPs
	conc.Recorrencias = app.Recorrencias{Store: store, PSPs: contas.PSPs, Clock: amb.Clock}
	conc.CobrancasRecorrentes = app.CobrancasRecorrentes{Store: store, PSPs: contas.PSPs, Clock: amb.Clock}
	if intervalo > 0 {
		rodando.Go(func() {
			jobs.Periodico(ctxJobs, "conciliação", intervalo, func(ctx context.Context) error {
				_, err := conc.Rodar(ctx)
				return err
			})
		})
	} else {
		slog.Warn("conciliação desligada: Pix sem aviso do banco não serão recuperados")
	}
	saida := app.Saida{Store: store, Vault: contas.Vault, Clock: amb.Clock, Enviador: notifier.NovoHTTP(amb.Versao)}
	if intervaloWebhook > 0 {
		rodando.Go(func() {
			jobs.Periodico(ctxJobs, "webhooks", intervaloWebhook, func(ctx context.Context) error {
				if _, err := saida.Distribuir(ctx); err != nil {
					return err
				}
				_, err := saida.Entregar(ctx)
				return err
			})
		})
	} else {
		slog.Warn("entrega de eventos desligada: os webhooks de saída ficam parados no outbox")
	}
	if intervaloCertificado > 0 {
		certs := app.Certificados{Store: store, Clock: amb.Clock}
		rodando.Go(func() {
			jobs.Periodico(ctxJobs, "vencimento de certificados", intervaloCertificado, func(ctx context.Context) error {
				n, err := certs.AvisarVencimentos(ctx)
				if n > 0 {
					slog.InfoContext(ctx, "certificado perto de vencer: evento certificado.expirando publicado", "contas", n)
				}
				return err
			})
		})
	} else {
		slog.Warn("aviso de vencimento de certificado desligado: ninguém será avisado antes de o certificado vencer")
	}
	rotas := http.NewServeMux()
	rotas.Handle("/painel/", web.Painel())
	rotas.Handle("GET /painel", http.RedirectHandler("/painel/", http.StatusMovedPermanently))
	rotas.Handle("/", httpapi.NewRouter(httpapi.Opcoes{
		Versao:     amb.Versao,
		Cobrancas:  app.Cobrancas{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
		CobrancasV: app.CobrancasV{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
		Contas:     contas,
		Webhooks: app.Webhooks{Store: store, PSPs: contas.PSPs, Recebimentos: app.Recebimentos{Store: store, Clock: amb.Clock},
			Recorrencias:         app.Recorrencias{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
			CobrancasRecorrentes: app.CobrancasRecorrentes{Store: store, PSPs: contas.PSPs, Clock: amb.Clock}},
		Recorrencias:         app.Recorrencias{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
		CobrancasRecorrentes: app.CobrancasRecorrentes{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
		Devolucoes:           app.Devolucoes{Store: store, PSPs: contas.PSPs, Clock: amb.Clock},
		APIKeys:              app.APIKeys{Store: store, Clock: amb.Clock},
		Idempotencia:         app.Idempotencia{Store: store, Clock: amb.Clock},
		Saida:                saida,
		Clock:                amb.Clock,
	}))
	srv := &http.Server{
		Addr:              addr,
		Handler:           rotas,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		// ctx já foi cancelado; o desligamento herda os valores dele, mas com prazo próprio.
		fim, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelar()
		_ = srv.Shutdown(fim)
	}()
	slog.Info("vintem no ar", "addr", addr, "version", amb.Versao)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// escopos é a flag repetível --escopo.
type escopos []domain.Escopo

func (e *escopos) String() string { return fmt.Sprint(*e) }

func (e *escopos) Set(v string) error {
	*e = append(*e, domain.Escopo(v))
	return nil
}

func apikey(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: vintem apikey criar|listar|revogar")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	keys := app.APIKeys{Store: contas.Store, Clock: amb.Clock}

	switch args[0] {
	case "criar":
		fs := flag.NewFlagSet("apikey criar", flag.ContinueOnError)
		fs.SetOutput(amb.Saida)
		var nome, conta string
		var es escopos
		fs.StringVar(&nome, "nome", "", "para que serve a chave (ex.: loja-producao)")
		fs.StringVar(&conta, "conta", "", "restringe a chave a uma conta")
		fs.Var(&es, "escopo", "escopo concedido (repetível)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		k, token, err := keys.Criar(ctx, nome, es, conta)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(amb.Saida, "api key %s criada com %v\n\n  %s\n\nGuarde agora: este token não aparece de novo.\n", k.ID, k.Escopos, token)
		return err
	case "listar":
		lista, err := keys.Listar(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNOME\tCONTA\tESCOPOS\tCRIADA\tREVOGADA")
		for _, k := range lista {
			revogada := "-"
			if k.RevogadaEm != nil {
				revogada = k.RevogadaEm.Format(time.DateTime)
			}
			conta := k.ContaID
			if conta == "" {
				conta = "(todas)"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\t%s\n", k.ID, k.Nome, conta, k.Escopos, k.CriadaEm.Format(time.DateTime), revogada)
		}
		return tw.Flush()
	case "revogar":
		if len(args) != 2 {
			return errors.New("use: vintem apikey revogar <id>")
		}
		if err := keys.Revogar(ctx, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(amb.Saida, "api key %s revogada\n", args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use criar, listar ou revogar)", args[0])
	}
}

// duracao lê uma variável no formato do Go (5m, 2h, 30s).
func duracao(amb Ambiente, chave, padrao string) (time.Duration, error) {
	v := getenv(amb, chave, padrao)
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("%s=%q: use uma duração como 5m ou 2h", chave, v)
	}
	return d, nil
}

// conciliacao monta o caso de uso sem Store; quem chama completa com o banco aberto.
func conciliacao(amb Ambiente) (app.Conciliacao, error) {
	janela, err := duracao(amb, "VINTEM_CONCILIACAO_JANELA", "2h")
	if err != nil {
		return app.Conciliacao{}, err
	}
	if janela == 0 {
		return app.Conciliacao{}, errors.New("VINTEM_CONCILIACAO_JANELA não pode ser zero")
	}
	return app.Conciliacao{Recebimentos: app.Recebimentos{Clock: amb.Clock}, Clock: amb.Clock, Janela: janela}, nil
}

func conciliar(ctx context.Context, amb Ambiente) error {
	conc, err := conciliacao(amb)
	if err != nil {
		return err
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	conc.Store, conc.Recebimentos.Store, conc.PSPs = contas.Store, contas.Store, contas.PSPs
	conc.Recorrencias = app.Recorrencias{Store: contas.Store, PSPs: contas.PSPs, Clock: amb.Clock}
	conc.CobrancasRecorrentes = app.CobrancasRecorrentes{Store: contas.Store, PSPs: contas.PSPs, Clock: amb.Clock}
	r, err := conc.Rodar(ctx)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(amb.Saida, "%d contas conciliadas, %d Pix recuperados, %d recorrências atualizadas, %d cobranças recorrentes atualizadas, %d devoluções atualizadas, %d falhas\n", r.Contas, r.Novos, r.Recorrencias, r.CobrancasRecorrentes, r.Devolucoes, r.Falhas)
	if err == nil && r.Falhas > 0 {
		err = fmt.Errorf("%d contas falharam (detalhes no log)", r.Falhas)
	}
	return err
}

func destino(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: vintem destino criar|listar|remover")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	saida := app.Saida{Store: contas.Store, Vault: contas.Vault, Clock: amb.Clock}
	switch args[0] {
	case "criar":
		fs := flag.NewFlagSet("destino criar", flag.ContinueOnError)
		fs.SetOutput(amb.Saida)
		endereco := fs.String("url", "", "URL da sua aplicação que recebe os eventos")
		conta := fs.String("conta", "", "só os eventos desta conta (vazio: todas)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		d, whsec, err := saida.ConfigurarDestino(ctx, *conta, *endereco)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(amb.Saida, "destino %s criado para %s\n\n  segredo: %s\n\nConfigure o segredo na sua aplicação para verificar a assinatura (Standard Webhooks).\nGuarde agora: ele não aparece de novo.\n", d.ID, d.URL, whsec)
		return err
	case "listar":
		lista, err := contas.Store.Destinos().Listar(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tCONTA\tURL\tCRIADO")
		for _, d := range lista {
			conta := d.ContaID
			if conta == "" {
				conta = "(todas)"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.ID, conta, d.URL, d.CriadoEm.Format(time.DateTime))
		}
		return tw.Flush()
	case "remover":
		if len(args) != 2 {
			return errors.New("use: vintem destino remover <id>")
		}
		if err := contas.Store.Destinos().Remover(ctx, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(amb.Saida, "destino %s removido\n", args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use criar, listar ou remover)", args[0])
	}
}

func dlq(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: vintem dlq listar|reenviar")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	saida := app.Saida{Store: contas.Store, Vault: contas.Vault, Clock: amb.Clock}
	switch args[0] {
	case "listar":
		lista, err := saida.DLQ(ctx, 200)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "EVENTO\tTIPO\tDESTINO\tTENTATIVAS\tÚLTIMO ERRO\tEVENTO EM")
		for _, e := range lista {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n", e.EventoID, e.Tipo, e.DestinoID, e.Tentativas, e.UltimoErro, e.CriadaEm.Format(time.DateTime))
		}
		return tw.Flush()
	case "reenviar":
		if len(args) != 2 {
			return errors.New("use: vintem dlq reenviar <evento-id>")
		}
		n, err := saida.Reenviar(ctx, args[1])
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("nenhuma entrega do evento %s está na DLQ", args[1])
		}
		_, err = fmt.Fprintf(amb.Saida, "%d entrega(s) do evento %s de volta à fila\n", n, args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use listar ou reenviar)", args[0])
	}
}
