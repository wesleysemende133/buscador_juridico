package crawler

import (
	"regexp"
	"strings"
)

// ============================================
// PADRÕES PARA NORMALIZAR TEXTO OCR
// ============================================

var (
	// ARTIGO LL → ARTIGO 11 (OCR lê 1 como L)
	reLL = regexp.MustCompile(`\bARTIGO\s+LL\b`)

	// ARTIGO L → ARTIGO 1
	reL = regexp.MustCompile(`\bARTIGO\s+L\b`)

	// ARTIGO II → ARTIGO 2
	reII = regexp.MustCompile(`\bARTIGO\s+II\b`)

	// ARTIGO III → ARTIGO 3
	reIII = regexp.MustCompile(`\bARTIGO\s+III\b`)

	// ARTIGO 1.º → ARTIGO 1
	reOrdinal = regexp.MustCompile(`(?i)\bARTIGO\s+(\d+)\.?\s*[º°]?`)

	// Corrigir OCR comuns
	reCfletal = regexp.MustCompile(`(?i)cfletal`)
	reLingua  = regexp.MustCompile(`(?i)lingua`)

	// Linhas vazias múltiplas
	reLinhasVazias = regexp.MustCompile(`\n{3,}`)
)

// ============================================
// NORMALIZAR TEXTO OCR
// ============================================

// NormalizarTextoOCR limpa o texto extraído por OCR
// para ser compatível com o regex extractor
func NormalizarTextoOCR(texto string) string {
	// 1. Corrigir OCR: LL → 11, L → 1, II → 2, III → 3
	texto = reLL.ReplaceAllString(texto, "ARTIGO 11")
	texto = reL.ReplaceAllString(texto, "ARTIGO 1")
	texto = reII.ReplaceAllString(texto, "ARTIGO 2")
	texto = reIII.ReplaceAllString(texto, "ARTIGO 3")

	// 2. Normalizar ARTIGO 1.º → ARTIGO 1
	texto = reOrdinal.ReplaceAllString(texto, "ARTIGO $1")

	// 3. Corrigir palavras comuns de OCR
	texto = reCfletal.ReplaceAllString(texto, "oficial")
	texto = reLingua.ReplaceAllString(texto, "Língua")

	// 4. Remover linhas vazias múltiplas
	texto = reLinhasVazias.ReplaceAllString(texto, "\n\n")

	return strings.TrimSpace(texto)
}
