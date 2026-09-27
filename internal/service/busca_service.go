package service

import (
	"strings"
	"unicode"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type BuscaService struct {
	buscador repository.Buscador
}

func NewBuscaService(buscador repository.Buscador) *BuscaService {
	return &BuscaService{buscador: buscador}
}

// ============================================
// NORMALIZAR TEXTO (remove acentos, minúsculas)
// ============================================

func normalizar(texto string) string {
	texto = strings.ToLower(texto)
	texto = strings.ReplaceAll(texto, "á", "a")
	texto = strings.ReplaceAll(texto, "à", "a")
	texto = strings.ReplaceAll(texto, "ã", "a")
	texto = strings.ReplaceAll(texto, "â", "a")
	texto = strings.ReplaceAll(texto, "ä", "a")
	texto = strings.ReplaceAll(texto, "é", "e")
	texto = strings.ReplaceAll(texto, "è", "e")
	texto = strings.ReplaceAll(texto, "ê", "e")
	texto = strings.ReplaceAll(texto, "ë", "e")
	texto = strings.ReplaceAll(texto, "í", "i")
	texto = strings.ReplaceAll(texto, "ì", "i")
	texto = strings.ReplaceAll(texto, "î", "i")
	texto = strings.ReplaceAll(texto, "ï", "i")
	texto = strings.ReplaceAll(texto, "ó", "o")
	texto = strings.ReplaceAll(texto, "ò", "o")
	texto = strings.ReplaceAll(texto, "õ", "o")
	texto = strings.ReplaceAll(texto, "ô", "o")
	texto = strings.ReplaceAll(texto, "ö", "o")
	texto = strings.ReplaceAll(texto, "ú", "u")
	texto = strings.ReplaceAll(texto, "ù", "u")
	texto = strings.ReplaceAll(texto, "û", "u")
	texto = strings.ReplaceAll(texto, "ü", "u")
	texto = strings.ReplaceAll(texto, "ç", "c")
	texto = strings.ReplaceAll(texto, "ñ", "n")
	
	// Remover pontuação
	texto = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			return r
		}
		return ' '
	}, texto)
	
	// Remover espaços múltiplos
	texto = strings.Join(strings.Fields(texto), " ")
	
	return texto
}

// ============================================
// SINÓNIMOS JURÍDICOS
// ============================================

var sinonimos = map[string][]string{
	// Trabalho
	"trabalho":    {"emprego", "laboral", "labor", "trabalhador", "empregado"},
	"emprego":     {"trabalho", "laboral", "trabalhador"},
	"trabalhador": {"empregado", "operário", "funcionário", "colaborador"},
	"salario":     {"remuneração", "vencimento", "pagamento", "ordenado"},
	"remuneracao": {"salário", "vencimento", "pagamento"},
	"ferias":      {"descanso", "recesso"},
	"despedimento": {"demissão", "rescisão", "dispensa"},
	
	// Família
	"familia":     {"casamento", "filhos", "cônjuges", "parentes"},
	"casamento":   {"matrimónio", "união", "família"},
	"divorcio":    {"separação", "dissolução"},
	"guarda":      {"custódia", "tutela", "filhos"},
	"filhos":      {"crianças", "menores", "descendentes"},
	"pensao":      {"alimentos", "remuneração"},
	"alimentos":   {"pensão", "sustento"},
	"heranca":     {"sucessão", "testamento", "espólio"},
	
	// Penal
	"crime":       {"delito", "infração", "ofensa"},
	"pena":        {"punição", "castigo", "sanção"},
	"prisao":      {"detenção", "cadeia", "reclusão"},
	"terrorismo":  {"terror", "terrorista"},
	"branqueamento": {"lavagem", "branqueamento de capitais"},
	"corrupcao":   {"suborno", "propina", "peculato"},
	
	// Comercial
	"empresa":     {"sociedade", "companhia", "firma"},
	"sociedade":   {"empresa", "companhia"},
	"banco":       {"instituição financeira", "crédito"},
	"credito":     {"empréstimo", "financiamento"},
	
	// Civil
	"contrato":    {"acordo", "convenção", "pacto"},
	"propriedade": {"posse", "domínio", "bem"},
	"obrigacao":   {"dever", "responsabilidade"},
	
	// Administrativo
	"tribunal":    {"juízo", "corte", "justiça"},
	"juiz":        {"magistrado", "julgador"},
	"administracao": {"governo", "gestão"},
	"imposto":     {"taxa", "tributo"},
	
	// Processual
	"processo":    {"ação", "causa", "litígio"},
	"recurso":     {"apelação", "agravo"},
	"prova":       {"evidência", "testemunho"},
}

// ============================================
// EXPANDIR QUERY COM SINÓNIMOS
// ============================================

func expandirComSinonimos(query string) []string {
	queryNorm := normalizar(query)
	palavras := strings.Fields(queryNorm)
	
	// Conjunto de termos a procurar
	termos := make(map[string]bool)
	
	// Adicionar a query completa
	termos[queryNorm] = true
	
	// Adicionar cada palavra
	for _, p := range palavras {
		termos[p] = true
		
		// Adicionar sinónimos
		if sins, ok := sinonimos[p]; ok {
			for _, s := range sins {
				termos[s] = true
			}
		}
	}
	
	// Converter para slice
	var resultado []string
	for t := range termos {
		resultado = append(resultado, t)
	}
	
	return resultado
}

// ============================================
// CALCULAR SCORE DE RELEVÂNCIA
// ============================================

func calcularScore(artigo domain.Artigo, termos []string, queryOriginal string) int {
	score := 0
	
	// Normalizar campos
	texto := normalizar(artigo.Texto)
	lei := normalizar(artigo.Lei)
	artigoNum := normalizar(artigo.Artigo)
	categoria := normalizar(artigo.Categoria)
	subcategoria := normalizar(artigo.Subcategoria)
	queryNorm := normalizar(queryOriginal)
	
	// ============================================
	// 1. MATCH EXATO NA QUERY COMPLETA (MAIOR SCORE)
	// ============================================
	if strings.Contains(lei, queryNorm) {
		score += 100 // Match no nome da lei
	}
	if strings.Contains(categoria, queryNorm) {
		score += 80 // Match na categoria
	}
	if strings.Contains(subcategoria, queryNorm) {
		score += 70 // Match na subcategoria
	}
	if strings.Contains(texto, queryNorm) {
		score += 60 // Match no texto
	}
	if strings.Contains(artigoNum, queryNorm) {
		score += 90 // Match no número do artigo
	}
	
	// ============================================
	// 2. MATCH POR PALAVRAS INDIVIDUAIS
	// ============================================
	for _, termo := range termos {
		if termo == "" {
			continue
		}
		
		// No nome da lei (peso alto)
		if strings.Contains(lei, termo) {
			score += 20
		}
		// Na categoria
		if strings.Contains(categoria, termo) {
			score += 15
		}
		// Na subcategoria
		if strings.Contains(subcategoria, termo) {
			score += 15
		}
		// No texto
		if strings.Contains(texto, termo) {
			score += 5
		}
		// Nas palavras-chave
		for _, pk := range artigo.PalavrasChave {
			if strings.Contains(normalizar(pk), termo) {
				score += 10
				break
			}
		}
	}
	
	return score
}

// ============================================
// BUSCA INTELIGENTE
// ============================================

func (s *BuscaService) BuscarInteligente(query string, categoria string, limite int) ([]domain.Artigo, error) {
	if limite <= 0 {
		limite = 20
	}
	
	// 1. Obter todos os artigos
	todos, err := s.buscador.ListarTodos()
	if err != nil {
		return []domain.Artigo{}, err
	}
	
	if len(todos) == 0 {
		return []domain.Artigo{}, nil
	}
	
	// 2. Filtrar por categoria (se especificada)
	if categoria != "" {
		var filtrados []domain.Artigo
		catNorm := normalizar(categoria)
		for _, a := range todos {
			if normalizar(a.Categoria) == catNorm {
				filtrados = append(filtrados, a)
			}
		}
		todos = filtrados
	}
	
	if len(todos) == 0 {
		return []domain.Artigo{}, nil
	}
	
	// 3. Expandir query com sinónimos
	termos := expandirComSinonimos(query)
	
	// 4. Calcular score para cada artigo
	type ArtigoComScore struct {
		Artigo domain.Artigo
		Score  int
	}
	
	var comScore []ArtigoComScore
	for _, a := range todos {
		score := calcularScore(a, termos, query)
		if score > 0 {
			comScore = append(comScore, ArtigoComScore{Artigo: a, Score: score})
		}
	}
	
	// 5. Ordenar por score (maior primeiro)
	for i := 0; i < len(comScore)-1; i++ {
		for j := i + 1; j < len(comScore); j++ {
			if comScore[i].Score < comScore[j].Score {
				comScore[i], comScore[j] = comScore[j], comScore[i]
			}
		}
	}
	
	// 6. Limitar
	if len(comScore) > limite {
		comScore = comScore[:limite]
	}
	
	// 7. Extrair apenas os artigos
	var resultado []domain.Artigo
	for _, item := range comScore {
		resultado = append(resultado, item.Artigo)
	}
	
	if resultado == nil {
		resultado = []domain.Artigo{}
	}
	
	return resultado, nil
}

// ============================================
// BUSCA SIMPLES (compatibilidade)
// ============================================

func (s *BuscaService) Buscar(query string, limite int) ([]domain.Artigo, error) {
	return s.BuscarInteligente(query, "", limite)
}

func (s *BuscaService) BuscarComFiltros(query string, categoria string, limite int) ([]domain.Artigo, error) {
	return s.BuscarInteligente(query, categoria, limite)
}

func (s *BuscaService) BuscarPorID(id string) (*domain.Artigo, error) {
	return s.buscador.BuscarPorID(id)
}

func (s *BuscaService) ListarTodos() ([]domain.Artigo, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return []domain.Artigo{}, err
	}
	if artigos == nil {
		return []domain.Artigo{}, nil
	}
	return artigos, nil
}

func (s *BuscaService) ListarLeis() ([]string, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return []string{}, err
	}

	leisMap := make(map[string]bool)
	for _, a := range artigos {
		if a.Lei != "" {
			leisMap[a.Lei] = true
		}
	}

	var leis []string
	for lei := range leisMap {
		leis = append(leis, lei)
	}
	if leis == nil {
		leis = []string{}
	}
	return leis, nil
}

func (s *BuscaService) ListarCategorias() ([]string, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return []string{}, err
	}

	catsMap := make(map[string]bool)
	for _, a := range artigos {
		if a.Categoria != "" {
			catsMap[a.Categoria] = true
		}
	}

	var cats []string
	for cat := range catsMap {
		cats = append(cats, cat)
	}
	if cats == nil {
		cats = []string{}
	}
	return cats, nil
}
