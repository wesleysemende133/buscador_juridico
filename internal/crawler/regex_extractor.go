package crawler

import (
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

// ============================================
// EXTRATOR INTELIGENTE POR REGEX
// ============================================

type RegexExtractor struct {
	// Padrões para encontrar artigos (múltiplos formatos)
	padroesArtigo []*regexp.Regexp

	// Padrões para encontrar o nome da lei
	padroesLei []*regexp.Regexp

	// Padrões para encontrar a data
	padroesData []*regexp.Regexp

	// Padrões para limpar o texto
	limpezaRegex *regexp.Regexp

	// Cache de artigos para evitar duplicados
	artigosVistos map[string]bool
}

func NewRegexExtractor() *RegexExtractor {
	return &RegexExtractor{
		// ============================================
		// PADRÕES PARA ENCONTRAR ARTIGOS
		// ============================================
		padroesArtigo: []*regexp.Regexp{
			// Padrão 1: "Art. 1º" ou "Art. 1°" (com grau)
			regexp.MustCompile(`(?i)(?:^|\n)\s*art(?:igo)?\.?\s*(\d+)\s*[º°]\s*`),
			// Padrão 2: "Art. 1." ou "Art. 1" (sem grau, com ponto)
			regexp.MustCompile(`(?i)(?:^|\n)\s*art(?:igo)?\.?\s*(\d+)\s*\.\s*`),
			// Padrão 3: "Art. 1.583" (com pontos no número)
			regexp.MustCompile(`(?i)(?:^|\n)\s*art(?:igo)?\.?\s*([\d\.]+)\s*[-–—]?\s*`),
			// Padrão 4: "Artigo 1º" (por extenso)
			regexp.MustCompile(`(?i)(?:^|\n)\s*artigo\s+(\d+)\s*[º°]?\s*`),
			// Padrão 5: "Art. 1-A" (com letra)
			regexp.MustCompile(`(?i)(?:^|\n)\s*art(?:igo)?\.?\s*(\d+[-]?[A-Z]?)\s*[º°\.]?\s*`),
			// Padrão 6: "ART. 1" (tudo maiúsculo)
			regexp.MustCompile(`(?:^|\n)\s*ART\.?\s*(\d+)\s*[º°\.]?\s*`),
			// Padrão 7: "Art. 1.º" (ordinal)
			regexp.MustCompile(`(?i)(?:^|\n)\s*art(?:igo)?\.?\s*(\d+)\s*\.\s*[º°]\s*`),
		},

		// ============================================
		// PADRÕES PARA ENCONTRAR O NOME DA LEI
		// ============================================
		padroesLei: []*regexp.Regexp{
			// "LEI N.º 11/2024 DE 07 DE JUNHO"
			regexp.MustCompile(`(?i)LEI\s+N[\.º°o]*\s*(\d+[/\-]\d+)\s*(?:DE\s+(\d{1,2})\s+DE\s+([A-ZÇÃÕÉÊÍÓÚ]+))?`),
			// "Lei n.º 8/2024"
			regexp.MustCompile(`(?i)LEI\s+N[\.º°o]*\s*(\d+/\d+)`),
			// "Decreto-Lei n.º 1/2024"
			regexp.MustCompile(`(?i)DECRETO[- ]LEI\s+N[\.º°o]*\s*(\d+[/\-]\d+)`),
			// "Decreto n.º 40/93"
			regexp.MustCompile(`(?i)DECRETO\s+N[\.º°o]*\s*(\d+[/\-]\d+)`),
			// "Resolução n.º 1/CSMJ/P/2009"
			regexp.MustCompile(`(?i)RESOLU[CÇ][AÃ]O\s+N[\.º°o]*\s*([\d/]+)`),
			// "Portaria n.º 123/2024"
			regexp.MustCompile(`(?i)PORTARIA\s+N[\.º°o]*\s*(\d+[/\-]\d+)`),
			// "Lei 19/97"
			regexp.MustCompile(`(?i)LEI\s+(\d+/\d+)`),
			// "Lei n.º 15/2023"
			regexp.MustCompile(`(?i)LEI\s+N[\.º°o]*\s*(\d+/\d+)`),
		},

		// ============================================
		// PADRÕES PARA ENCONTRAR DATAS
		// ============================================
		padroesData: []*regexp.Regexp{
			// "de 20 de Setembro de 2024"
			regexp.MustCompile(`(?i)(\d{1,2})\s+de\s+(janeiro|fevereiro|março|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\s+de\s+(\d{4})`),
			// "20/09/2024"
			regexp.MustCompile(`(\d{1,2})/(\d{1,2})/(\d{4})`),
			// "2024-09-20"
			regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})`),
		},

		// ============================================
		// REGEX DE LIMPEZA
		// ============================================
		limpezaRegex: regexp.MustCompile(`\s+`),

		artigosVistos: make(map[string]bool),
	}
}

// ============================================
// EXTRAIR ARTIGOS (FUNÇÃO PRINCIPAL)
// ============================================

func (r *RegexExtractor) ExtrairArtigos(texto string, fonte string) ([]domain.Artigo, error) {
	log.Printf("📖 Extraindo artigos com REGEX INTELIGENTE de: %s (tamanho: %d)", fonte, len(texto))

	// 1. Limpar o texto
	texto = r.limparTexto(texto)

	// 2. Identificar a lei (com múltiplos padrões)
	leiNome, leiNumero := r.extrairNomeLei(texto, fonte)
	log.Printf("📜 Lei identificada: %s (%s)", leiNome, leiNumero)

	// 3. Identificar a data
	dataVigencia := r.extrairData(texto)

	// 4. Extrair ementa (resumo)
	ementa := r.extrairEmenta(texto)

	// 5. Encontrar todos os artigos (com múltiplos padrões)
	artigos := r.encontrarArtigosInteligente(texto, leiNome, leiNumero, fonte, dataVigencia, ementa)

	// 6. Remover duplicados
	artigos = r.removerDuplicados(artigos)

	// 7. Ordenar por número do artigo
	sort.Slice(artigos, func(i, j int) bool {
		return r.extrairNumeroInt(artigos[i].Artigo) < r.extrairNumeroInt(artigos[j].Artigo)
	})

	log.Printf("✅ Regex extraiu %d artigos únicos de %s", len(artigos), fonte)
	return artigos, nil
}

// ============================================
// LIMPAR TEXTO
// ============================================

func (r *RegexExtractor) limparTexto(texto string) string {
	// Normalizar quebras de linha
	texto = strings.ReplaceAll(texto, "\r\n", "\n")
	texto = strings.ReplaceAll(texto, "\r", "\n")

	// Remover caracteres estranhos
	texto = strings.Map(func(c rune) rune {
		if c == '\n' || c == '\t' || unicode.IsPrint(c) {
			return c
		}
		return -1
	}, texto)

	// Reduzir espaços múltiplos
	texto = regexp.MustCompile(`[ \t]+`).ReplaceAllString(texto, " ")
	texto = regexp.MustCompile(`\n{3,}`).ReplaceAllString(texto, "\n\n")

	return texto
}

// ============================================
// EXTRAIR NOME DA LEI (INTELIGENTE)
// ============================================

func (r *RegexExtractor) extrairNomeLei(texto, fonte string) (string, string) {
	// Procurar nas primeiras 5000 letras (mais contexto)
	inicio := texto
	if len(texto) > 5000 {
		inicio = texto[:5000]
	}

	// Tentar cada padrão
	for _, padrao := range r.padroesLei {
		matches := padrao.FindStringSubmatch(inicio)
		if len(matches) >= 2 {
			numero := matches[1]
			tipo := r.determinarTipoLei(matches[0])
			return tipo, numero
		}
	}

	// Fallback: procurar qualquer menção de "Lei", "Decreto", etc.
	fallback := regexp.MustCompile(`(?i)(lei|decreto|resolução|portaria)\s+(\d+[/\-]\d+)`)
	matches := fallback.FindStringSubmatch(inicio)
	if len(matches) >= 3 {
		return strings.Title(strings.ToLower(matches[1])), matches[2]
	}

	return fonte, ""
}

// ============================================
// DETERMINAR TIPO DE LEI
// ============================================

func (r *RegexExtractor) determinarTipoLei(texto string) string {
	upper := strings.ToUpper(texto)
	switch {
	case strings.Contains(upper, "DECRETO-LEI"):
		return "Decreto-Lei"
	case strings.Contains(upper, "DECRETO"):
		return "Decreto"
	case strings.Contains(upper, "RESOLU"):
		return "Resolução"
	case strings.Contains(upper, "PORTARIA"):
		return "Portaria"
	case strings.Contains(upper, "DESPACHO"):
		return "Despacho"
	case strings.Contains(upper, "REGULAMENTO"):
		return "Regulamento"
	default:
		return "Lei"
	}
}

// ============================================
// EXTRAIR DATA (MÚLTIPLOS FORMATOS)
// ============================================

func (r *RegexExtractor) extrairData(texto string) time.Time {
	// Tentar cada padrão
	for _, padrao := range r.padroesData {
		matches := padrao.FindStringSubmatch(texto)
		if len(matches) >= 4 {
			// Formato textual: "20 de Setembro de 2024"
			if len(matches[2]) > 2 {
				return r.parseDataTextual(matches[1], matches[2], matches[3])
			}
			// Formato numérico: "20/09/2024"
			return r.parseDataNumerica(matches[1], matches[2], matches[3])
		}
	}

	return time.Now()
}

func (r *RegexExtractor) parseDataTextual(dia, mes, ano string) time.Time {
	meses := map[string]string{
		"janeiro": "01", "fevereiro": "02", "março": "03", "marco": "03",
		"abril": "04", "maio": "05", "junho": "06",
		"julho": "07", "agosto": "08", "setembro": "09",
		"outubro": "10", "novembro": "11", "dezembro": "12",
	}

	if m, ok := meses[strings.ToLower(mes)]; ok {
		dataStr := fmt.Sprintf("%s-%s-%02s", ano, m, dia)
		if t, err := time.Parse("2006-01-02", dataStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func (r *RegexExtractor) parseDataNumerica(a, b, c string) time.Time {
	// Tentar DD/MM/YYYY
	if t, err := time.Parse("02/01/2006", fmt.Sprintf("%s/%s/%s", a, b, c)); err == nil {
		return t
	}
	// Tentar YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", fmt.Sprintf("%s-%s-%s", a, b, c)); err == nil {
		return t
	}
	return time.Now()
}

// ============================================
// EXTRAIR EMENTA (RESUMO)
// ============================================

func (r *RegexExtractor) extrairEmenta(texto string) string {
	// Procurar por "SUMÁRIO" ou "Assunto:"
	padroes := []*regexp.Regexp{
		regexp.MustCompile(`(?i)SUM[ÁA]RIO[:\s]+([^\n]{20,500})`),
		regexp.MustCompile(`(?i)ASSUNTO[:\s]+([^\n]{20,500})`),
		regexp.MustCompile(`(?i)OBJETO[:\s]+([^\n]{20,500})`),
	}

	for _, padrao := range padroes {
		matches := padrao.FindStringSubmatch(texto)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// ============================================
// ENCONTRAR ARTIGOS (INTELIGENTE)
// ============================================

func (r *RegexExtractor) encontrarArtigosInteligente(texto, leiNome, leiNumero, fonte string, dataVigencia time.Time, ementa string) []domain.Artigo {
	var artigos []domain.Artigo

	// Tentar cada padrão e escolher o que encontra mais artigos
	melhorPadrao := -1
	melhorContagem := 0

	for i, padrao := range r.padroesArtigo {
		matches := padrao.FindAllStringSubmatchIndex(texto, -1)
		if len(matches) > melhorContagem {
			melhorContagem = len(matches)
			melhorPadrao = i
		}
	}

	if melhorPadrao == -1 {
		log.Println("⚠️ Nenhum padrão de artigo encontrado")
		return artigos
	}

	log.Printf("🎯 Padrão %d escolhido (%d artigos encontrados)", melhorPadrao+1, melhorContagem)

	// Usar o melhor padrão
	padrao := r.padroesArtigo[melhorPadrao]
	matches := padrao.FindAllStringSubmatchIndex(texto, -1)

	for i, match := range matches {
		// Extrair número
		numero := texto[match[2]:match[3]]
		numero = r.normalizarNumero(numero)

		if numero == "" {
			continue
		}

		// Início do texto do artigo
		inicio := match[1]

		// Fim do texto (próximo artigo ou fim)
		var fim int
		if i+1 < len(matches) {
			fim = matches[i+1][0]
		} else {
			fim = len(texto)
		}

		if inicio >= fim {
			continue
		}

		textoArtigo := strings.TrimSpace(texto[inicio:fim])
		textoArtigo = r.limparTextoArtigo(textoArtigo)

		// Validação
		if !r.validarArtigo(textoArtigo) {
			continue
		}

		// Criar artigo
		artigo := domain.Artigo{
			ID:             r.gerarID(leiNome, leiNumero, numero),
			Lei:            leiNome,
			LeiNumero:      leiNumero,
			Artigo:         fmt.Sprintf("Art. %s", numero),
			Texto:          textoArtigo,
			PalavrasChave:  r.extrairPalavrasChave(textoArtigo),
			Versao:         1,
			DataVigencia:   dataVigencia,
			DataPublicacao: dataVigencia,
			Status:         "Vigente",
			StatusMotivo:   ementa,
			Fonte:          fonte,
			CriadoEm:       time.Now(),
			AtualizadoEm:   time.Now(),
			AprovadoPor:    "Regex Inteligente",
		}

		artigos = append(artigos, artigo)
	}

	return artigos
}

// ============================================
// VALIDAR ARTIGO
// ============================================

func (r *RegexExtractor) validarArtigo(texto string) bool {
	// Muito curto ou muito longo
	if len(texto) < 30 || len(texto) > 5000 {
		return false
	}

	// Deve ter pelo menos 5 palavras
	palavras := strings.Fields(texto)
	if len(palavras) < 5 {
		return false
	}

	// Não pode ser só números
	sóNumeros := regexp.MustCompile(`^[\d\s\.\-]+$`)
	if sóNumeros.MatchString(texto) {
		return false
	}

	// Não pode ser cabeçalho/rodapé
	headers := []string{
		"Boletim da República",
		"Página",
		"I SÉRIE",
		"2º SUPLEMENTO",
		"www.ts.gov.mz",
	}
	for _, h := range headers {
		if strings.Contains(texto, h) && len(texto) < 100 {
			return false
		}
	}

	return true
}

// ============================================
// NORMALIZAR NÚMERO
// ============================================

func (r *RegexExtractor) normalizarNumero(numero string) string {
	numero = strings.TrimSpace(numero)
	numero = strings.TrimRight(numero, "º°-. ")
	numero = strings.TrimLeft(numero, ". ")
	return numero
}

// ============================================
// LIMPAR TEXTO DO ARTIGO
// ============================================

func (r *RegexExtractor) limparTextoArtigo(texto string) string {
	// Remover quebras de linha
	texto = strings.ReplaceAll(texto, "\n", " ")

	// Remover espaços múltiplos
	texto = regexp.MustCompile(`\s+`).ReplaceAllString(texto, " ")

	// Remover cabeçalhos/rodapés
	linhasIgnorar := []string{
		"Boletim da República",
		"Página",
		"I SÉRIE",
		"2º SUPLEMENTO",
		"www.ts.gov.mz",
		"Publicado no",
	}

	for _, ignorar := range linhasIgnorar {
		idx := strings.Index(texto, ignorar)
		if idx > 0 {
			texto = texto[:idx]
		}
	}

	// Remover numeração de página no final
	texto = regexp.MustCompile(`\b\d{1,3}\b\s*$`).ReplaceAllString(texto, "")

	return strings.TrimSpace(texto)
}

// ============================================
// REMOVER DUPLICADOS
// ============================================

func (r *RegexExtractor) removerDuplicados(artigos []domain.Artigo) []domain.Artigo {
	var unicos []domain.Artigo

	for _, a := range artigos {
		chave := fmt.Sprintf("%s|%s|%s", a.Lei, a.LeiNumero, a.Artigo)
		if !r.artigosVistos[chave] {
			r.artigosVistos[chave] = true
			unicos = append(unicos, a)
		}
	}

	return unicos
}

// ============================================
// EXTRAIR NÚMERO INTEIRO (PARA ORDENAR)
// ============================================

func (r *RegexExtractor) extrairNumeroInt(artigo string) int {
	// Extrair número de "Art. 1.583" → 1583
	re := regexp.MustCompile(`\d+`)
	partes := re.FindAllString(artigo, -1)
	if len(partes) == 0 {
		return 0
	}

	// Juntar todos os números
	numeroStr := strings.Join(partes, "")
	var numero int
	fmt.Sscanf(numeroStr, "%d", &numero)
	return numero
}

// ============================================
// EXTRAIR PALAVRAS-CHAVE
// ============================================

func (r *RegexExtractor) extrairPalavrasChave(texto string) []string {
	texto = strings.ToLower(texto)
	texto = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(texto, " ")

	palavras := strings.Fields(texto)

	stopwords := map[string]bool{
		"a": true, "o": true, "e": true, "da": true, "de": true, "do": true,
		"em": true, "com": true, "para": true, "por": true, "que": true,
		"se": true, "os": true, "as": true, "um": true, "uma": true,
		"ao": true, "pelo": true, "pela": true, "no": true, "na": true,
		"é": true, "são": true, "ser": true, "ter": true, "seu": true,
		"sua": true, "ou": true, "como": true, "mais": true, "dos": true,
		"das": true, "aos": true, "às": true,
	}

	var chaves []string
	vistas := make(map[string]bool)

	for _, p := range palavras {
		if len(p) > 4 && !stopwords[p] && !vistas[p] {
			chaves = append(chaves, p)
			vistas[p] = true
			if len(chaves) >= 15 {
				break
			}
		}
	}

	return chaves
}

// ============================================
// GERAR ID ÚNICO (MAIS ROBUSTO)
// ============================================

func (r *RegexExtractor) gerarID(lei, leiNumero, numero string) string {
	// Limpar lei
	nomeLei := strings.ToLower(lei)
	nomeLei = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(nomeLei, "_")
	nomeLei = strings.Trim(nomeLei, "_")

	// Limpar número
	numLimpo := strings.ReplaceAll(leiNumero, "/", "_")
	numLimpo = strings.ReplaceAll(numLimpo, ".", "_")
	numLimpo = strings.ReplaceAll(numLimpo, "-", "_")

	artLimpo := strings.ReplaceAll(numero, ".", "_")
	artLimpo = strings.ReplaceAll(artLimpo, "/", "_")

	id := fmt.Sprintf("%s_%s_art_%s", nomeLei, numLimpo, artLimpo)
	id = regexp.MustCompile(`_+`).ReplaceAllString(id, "_")

	return id
}
