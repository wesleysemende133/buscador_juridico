package crawler

// Fonte representa um site de onde o crawler extrai PDFs de leis
type Fonte struct {
	Nome       string
	URL        string
	Tipo       string // "tribunal", "boletim", "ministerio", "palop", "academico"
	Activa     bool
	Prioridade int // 1 = alta, 5 = baixa
	MaxDepth   int // profundidade de navegação
}

// Fontes de legislação moçambicana
var Fontes = []Fonte{
	// ============================================
	// 🏛️  IMPRENSA NACIONAL — Boletim da República
	// (certificado SSL inválido — requer InsecureSkipVerify)
	// ============================================
	{
		Nome:       "Imprensa Nacional - Boletim",
		URL:        "https://www.inm.gov.mz/pt-br/bulletin",
		Tipo:       "boletim",
		Activa:     true,
		Prioridade: 1,
		MaxDepth:   2,
	},
	{
		Nome:       "Imprensa Nacional - BR I Série",
		URL:        "https://www.inm.gov.mz/pt-br/tipo-de-produto/boletim-da-rep%C3%BAblica-i-serie",
		Tipo:       "boletim",
		Activa:     true,
		Prioridade: 1,
		MaxDepth:   2,
	},

	// ============================================
	// ⚖️  SAFLII — Códigos de Moçambique
	// (requer headers realistas)
	// ============================================
	{
		Nome:       "SAFLII - Códigos",
		URL:        "https://www.saflii.org/mz/legis/codigos/",
		Tipo:       "tribunal",
		Activa:     true,
		Prioridade: 2,
		MaxDepth:   2,
	},
	{
		Nome:       "SAFLII - Legislação",
		URL:        "https://www.saflii.org/mz/legis/",
		Tipo:       "tribunal",
		Activa:     true,
		Prioridade: 2,
		MaxDepth:   2,
	},

	// ============================================
	// 📚  Legis-PALOP+TL
	// (requer subscrição — desactivada)
	// ============================================
	{
		Nome:       "Legis-PALOP+TL",
		URL:        "https://www.legis-palop.org",
		Tipo:       "palop",
		Activa:     false,
		Prioridade: 3,
		MaxDepth:   2,
	},

	// ============================================
	// 🔴  Tribunal Supremo
	// (bloqueado por Cloudflare — desactivado)
	// ============================================
	{
		Nome:       "TS - Legislação",
		URL:        "https://www.ts.gov.mz/legislacao/",
		Tipo:       "tribunal",
		Activa:     false,
		Prioridade: 1,
		MaxDepth:   3,
	},
}

// FontesActivas devolve só as fontes ligadas
func FontesActivas() []Fonte {
	var activas []Fonte
	for _, f := range Fontes {
		if f.Activa {
			activas = append(activas, f)
		}
	}
	return activas
}
