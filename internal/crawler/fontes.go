package crawler

type Fonte struct {
	Nome       string
	URL        string
	Tipo       string
	Activa     bool
	Prioridade int
	MaxDepth   int
}

var Fontes = []Fonte{
	// ============================================
	// 🏛️  TRIBUNAL SUPREMO — 116 PDFs confirmados!
	// ============================================
	{
		Nome:       "TS - Legislação",
		URL:        "https://www.ts.gov.mz/legislacao/",
		Tipo:       "tribunal",
		Activa:     true,
		Prioridade: 1,
		MaxDepth:   3,
	},
	{
		Nome:       "TS - Página Principal",
		URL:        "https://www.ts.gov.mz/",
		Tipo:       "tribunal",
		Activa:     true,
		Prioridade: 1,
		MaxDepth:   2,
	},
}

func FontesActivas() []Fonte {
	var activas []Fonte
	for _, f := range Fontes {
		if f.Activa {
			activas = append(activas, f)
		}
	}
	return activas
}
