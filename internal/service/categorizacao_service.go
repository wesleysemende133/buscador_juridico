package service

import (
	"strings"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

// ============================================
// SERVIÇO DE CATEGORIZAÇÃO
// ============================================

type CategorizacaoService struct {
	// Mapeamento de leis conhecidas → categoria
	mapaLeis map[string]CategoriaInfo
	// Palavras-chave por categoria
	palavrasChave map[string][]string
}

type CategoriaInfo struct {
	Categoria    string
	Subcategoria string
}

func NewCategorizacaoService() *CategorizacaoService {
	return &CategorizacaoService{
		mapaLeis:      buildMapaLeis(),
		palavrasChave: buildPalavrasChave(),
	}
}

// ============================================
// MAPA DE LEIS CONHECIDAS
// ============================================

func buildMapaLeis() map[string]CategoriaInfo {
	return map[string]CategoriaInfo{
		// Direito do Trabalho
		"13/2023": {"Direito do Trabalho", "Lei Geral do Trabalho"},
		"23/2007": {"Direito do Trabalho", "Lei Geral do Trabalho"},

		// Direito Penal
		"24/2019": {"Direito Penal", "Código Penal"},
		"25/2019": {"Direito Processual Penal", "Código de Processo Penal"},
		"15/2023": {"Direito Penal", "Combate ao Terrorismo"},
		"14/2023": {"Direito Penal", "Branqueamento de Capitais"},
		"3/2024":  {"Direito Penal", "Branqueamento (Alterações)"},
		"4/2024":  {"Direito Penal", "Terrorismo (Alterações)"},
		"13/2020": {"Direito Penal", "Perda Alargada de Bens"},

		// Direito Comercial
		"20/2020": {"Direito Comercial", "Instituições de Crédito"},
		"9/2022":  {"Direito Comercial", "Direitos de Autor"},

		// Direito Administrativo
		"10/92":    {"Direito Administrativo", "Organização Judiciária"},
		"16/2024":  {"Direito Administrativo", "Magistrados Judiciais"},
		"7/2009":   {"Direito Administrativo", "Estatuto dos Magistrados"},
		"11/1979":  {"Direito Administrativo", "Segurança do Estado"},
		"29/75":    {"Direito Administrativo", "Organização Judiciária"},
		"12/78":    {"Direito Administrativo", "Organização Judiciária"},
		"24/2007":  {"Direito Administrativo", "Organização Judiciária"},
		"17/2013":  {"Direito Administrativo", "Estatuto dos Juízes"},
		"17/2020":  {"Direito Administrativo", "Código Penal (alterações)"},

		// Direito Civil
		"10.406/2002": {"Direito Civil", "Código Civil"},
		"19/97":       {"Direito Civil", "Lei de Terras"},

		// Direito Processual
		"26/2019": {"Direito Processual", "Execução das Penas"},

		// Direito Internacional
		"15/2024": {"Direito Internacional", "Extradição"},

		// Direito Constitucional
		"2004": {"Direito Constitucional", "Constituição"},

		// Fallback
		"2/2011":  {"Direito Civil", "Diversos"},
		"13/2022": {"Direito Ambiental", "Diversos"},
		"10/2022": {"Direito Marítimo", "Tribunais Marítimos"},
		"10/2005": {"Direito Administrativo", "Decreto"},
	}
}

// ============================================
// PALAVRAS-CHAVE POR CATEGORIA (FALLBACK)
// ============================================

func buildPalavrasChave() map[string][]string {
	return map[string][]string{
		"Direito do Trabalho": {
			"trabalhador", "empregador", "contrato de trabalho", "salário",
			"férias", "despedimento", "sindicato", "greve", "remuneração",
			"maternidade", "paternidade", "acidente de trabalho",
		},
		"Direito Penal": {
			"crime", "pena", "prisão", "arguido", "terrorismo",
			"branqueamento", "furto", "roubo", "homicídio", "corrupção",
			"contravenção", "infracção",
		},
		"Direito Civil": {
			"família", "casamento", "divórcio", "guarda", "filhos",
			"herança", "testamento", "propriedade", "contrato", "obrigação",
			"alimentos", "pensão",
		},
		"Direito Comercial": {
			"empresa", "sociedade", "comércio", "banco", "crédito",
			"instituição financeira", "título", "acção", "falência",
		},
		"Direito Administrativo": {
			"administração pública", "funcionário", "servidor", "concurso",
			"licitação", "tribunal", "juiz", "magistrado", "tributário",
			"imposto",
		},
		"Direito Processual": {
			"processo", "instrução", "julgamento", "sentença", "recurso",
			"tribunal", "juiz", "prova", "audiência", "acusação",
		},
		"Direito Constitucional": {
			"constituição", "direitos fundamentais", "soberania",
			"assembleia", "presidente", "governo",
		},
		"Direito Internacional": {
			"tratado", "convenção", "extradição", "cooperação internacional",
			"acordo",
		},
	}
}

// ============================================
// CATEGORIZAR UM ARTIGO
// ============================================

func (s *CategorizacaoService) Categorizar(artigo *domain.Artigo) {
	// 1. Tentar pelo número da lei (mais fiável)
	if info, ok := s.mapaLeis[artigo.LeiNumero]; ok {
		artigo.Categoria = info.Categoria
		artigo.Subcategoria = info.Subcategoria
		return
	}

	// 2. Tentar pelas palavras-chave do texto
	textoLower := strings.ToLower(artigo.Texto + " " + artigo.Lei)

	melhorCategoria := ""
	melhorContagem := 0

	for categoria, palavras := range s.palavrasChave {
		contagem := 0
		for _, palavra := range palavras {
			if strings.Contains(textoLower, palavra) {
				contagem++
			}
		}
		if contagem > melhorContagem {
			melhorContagem = contagem
			melhorCategoria = categoria
		}
	}

	// Se encontrou correspondência forte
	if melhorContagem >= 2 {
		artigo.Categoria = melhorCategoria
		artigo.Subcategoria = "Geral"
		return
	}

	// 3. Fallback
	artigo.Categoria = "Não Categorizado"
	artigo.Subcategoria = "Diversos"
}

// ============================================
// CATEGORIZAR VÁRIOS ARTIGOS
// ============================================

func (s *CategorizacaoService) CategorizarVarios(artigos []domain.Artigo) []domain.Artigo {
	for i := range artigos {
		s.Categorizar(&artigos[i])
	}
	return artigos
}

// ============================================
// LISTAR CATEGORIAS DISPONÍVEIS
// ============================================

func (s *CategorizacaoService) ListarCategorias() []string {
	categorias := make(map[string]bool)
	for _, info := range s.mapaLeis {
		categorias[info.Categoria] = true
	}
	for cat := range s.palavrasChave {
		categorias[cat] = true
	}

	var resultado []string
	for cat := range categorias {
		resultado = append(resultado, cat)
	}
	return resultado
}

// ============================================
// NORMALIZAR NOME DA LEI (AUMENTAR PRECISÃO)
// ============================================

func (s *CategorizacaoService) NormalizarNomeLei(lei string, numero string) string {
	// Limpar
	lei = strings.TrimSpace(lei)
	numero = strings.TrimSpace(numero)

	// Se já tem nome específico, manter
	if len(lei) > 10 && !strings.EqualFold(lei, "Lei") && !strings.EqualFold(lei, "Decreto") {
		return lei
	}

	// Tentar usar o mapa
	if info, ok := s.mapaLeis[numero]; ok {
		return "Lei " + numero + " - " + info.Subcategoria
	}

	return "Lei " + numero
}
