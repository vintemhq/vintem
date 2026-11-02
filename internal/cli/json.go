package cli

import (
	"encoding/json"
	"flag"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
)

// Saída em JSON dos comandos listar (#116): os mesmos campos da tabela, para
// script e agente não fazerem parse de texto. Os tipos são explícitos para
// nenhum segredo (hash da API key, credenciais, segredo do destino) vazar
// quando o domínio ganhar um campo novo.

type contaJSON struct {
	ID       string `json:"id"`
	Nome     string `json:"nome"`
	PSP      string `json:"psp"`
	ChavePix string `json:"chavePix"`
	Ambiente string `json:"ambiente"`
	Ativa    bool   `json:"ativa"`
	// CertificadoValidade é a data (AAAA-MM-DD) até quando o certificado vale; null sem certificado.
	CertificadoValidade *string `json:"certificadoValidade"`
}

type apikeyJSON struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	// ContaID vazio: a chave vale para todas as contas.
	ContaID    string          `json:"contaId"`
	Escopos    []domain.Escopo `json:"escopos"`
	CriadaEm   time.Time       `json:"criadaEm"`
	RevogadaEm *time.Time      `json:"revogadaEm"`
}

type destinoJSON struct {
	ID string `json:"id"`
	// ContaID vazio: o destino recebe os eventos de todas as contas.
	ContaID  string    `json:"contaId"`
	URL      string    `json:"url"`
	CriadoEm time.Time `json:"criadoEm"`
}

type dlqJSON struct {
	EventoID   string            `json:"eventoId"`
	Tipo       domain.TipoEvento `json:"tipo"`
	DestinoID  string            `json:"destinoId"`
	Tentativas int               `json:"tentativas"`
	UltimoErro string            `json:"ultimoErro"`
	CriadaEm   time.Time         `json:"criadaEm"`
}

// flagJSON lê as flags de um comando listar, que só aceita --json.
func flagJSON(nome string, args []string, amb Ambiente) (bool, error) {
	fs := flag.NewFlagSet(nome, flag.ContinueOnError)
	fs.SetOutput(amb.Saida)
	emJSON := fs.Bool("json", false, "imprime um array JSON em vez da tabela")
	if err := fs.Parse(args); err != nil {
		return false, err
	}
	return *emJSON, nil
}

// imprimirJSON escreve v indentado, terminado em quebra de linha.
func imprimirJSON(amb Ambiente, v any) error {
	enc := json.NewEncoder(amb.Saida)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
