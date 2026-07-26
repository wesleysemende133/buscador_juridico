package domain

import "time"

type Artigo struct {
	ID             string    `json:"id"`
	Lei            string    `json:"lei"`
	LeiNumero      string    `json:"lei_numero"`
	Artigo         string    `json:"artigo"`
	Texto          string    `json:"texto"`
	PalavrasChave  []string  `json:"palavras_chave"`
	Versao         int       `json:"versao"`
	DataVigencia   time.Time `json:"data_vigencia"`
	DataPublicacao time.Time `json:"data_publicacao"`
	Status         string    `json:"status"`
	StatusMotivo   string    `json:"status_motivo"`
	Fonte          string    `json:"fonte"`
	URL            string    `json:"url"`
	CriadoEm       time.Time `json:"criado_em"`
	AtualizadoEm   time.Time `json:"atualizado_em"`
	AprovadoPor    string    `json:"aprovado_por"`
}
