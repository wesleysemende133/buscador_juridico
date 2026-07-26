package crawler

import (
	"fmt"
	"log"
	"time"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

// BuscarLeisPlanalto busca leis (VERSÃO DE EXEMPLO)
// Para Moçambique, substitua pelas URLs reais:
// - https://www.legis-palop.org/bd/mz/
// - https://www.portaldogoverno.gov.mz/legislacao/
func BuscarLeisPlanalto() ([]domain.Artigo, error) {
	var artigos []domain.Artigo

	log.Println("🕷️  Iniciando crawler (exemplo para Moçambique)...")

	// ============================================
	// EXEMPLO: Artigos fictícios para demonstração
	// Em produção, substitua por scraping real
	// ============================================

	artigos = append(artigos, domain.Artigo{
		ID:             fmt.Sprintf("crawl_%d", time.Now().UnixNano()),
		Lei:            "Lei de Terras de Moçambique",
		LeiNumero:      "Lei 19/97",
		Artigo:         "Art. 1",
		Texto:          "A terra é propriedade do Estado e não pode ser vendida nem alienada.",
		PalavrasChave:  []string{"terras", "propriedade", "moçambique", "estado"},
		Versao:         1,
		DataVigencia:   time.Date(1997, 10, 1, 0, 0, 0, 0, time.UTC),
		DataPublicacao: time.Date(1997, 10, 1, 0, 0, 0, 0, time.UTC),
		Status:         "Vigente",
		StatusMotivo:   "",
		Fonte:          "Legis-PALOP+TL (Exemplo)",
		URL:            "https://www.legis-palop.org/bd/mz/",
		CriadoEm:       time.Now(),
		AtualizadoEm:   time.Now(),
		AprovadoPor:    "Crawler",
	})

	artigos = append(artigos, domain.Artigo{
		ID:             fmt.Sprintf("crawl_%d", time.Now().UnixNano()),
		Lei:            "Código do Trabalho de Moçambique",
		LeiNumero:      "Lei 23/2007",
		Artigo:         "Art. 1",
		Texto:          "O presente Código estabelece o regime jurídico das relações de trabalho em Moçambique.",
		PalavrasChave:  []string{"trabalho", "emprego", "moçambique", "relações trabalhistas"},
		Versao:         1,
		DataVigencia:   time.Date(2007, 8, 1, 0, 0, 0, 0, time.UTC),
		DataPublicacao: time.Date(2007, 8, 1, 0, 0, 0, 0, time.UTC),
		Status:         "Vigente",
		StatusMotivo:   "",
		Fonte:          "Legis-PALOP+TL (Exemplo)",
		URL:            "https://www.legis-palop.org/bd/mz/",
		CriadoEm:       time.Now(),
		AtualizadoEm:   time.Now(),
		AprovadoPor:    "Crawler",
	})

	log.Printf("✅ Crawler encontrou %d artigos (exemplo)", len(artigos))
	return artigos, nil
}
