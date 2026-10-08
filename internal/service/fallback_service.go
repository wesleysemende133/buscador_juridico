package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ============================================
// FALLBACK: CULTURA GERAL JURÍDICA (COM CAUTELA)
// ============================================

type TipoFallback string

const (
	FallbackConceitoUniversal TipoFallback = "conceito_universal"
	FallbackLeiEspecifica     TipoFallback = "lei_especifica"
	FallbackOutroPais         TipoFallback = "outro_pais"
	FallbackForaDireito       TipoFallback = "fora_direito"
	FallbackDesconhecido      TipoFallback = "desconhecido"
)

type RespostaFallback struct {
	Tipo       TipoFallback
	Resposta   string
	Confianca  float64
	AvisoLegal string
}

// ResponderComCulturaGeral responde com conhecimento jurídico geral
func (s *AgenteService) ResponderComCulturaGeral(
	ctx context.Context,
	pergunta string,
	modo string,
	historico []MensagemChat,
) (*RespostaFallback, error) {

	log.Printf("🎓 Fallback activado para: %q", pergunta)

	// Construir histórico
	var hist strings.Builder
	if len(historico) > 0 {
		hist.WriteString("CONTEXTO DA CONVERSA ANTERIOR:\n")
		for _, m := range historico {
			role := "Utilizador"
			if m.Role == "assistant" {
				role = "Assistente"
			}
			texto := m.Texto
			if len(texto) > 250 {
				texto = texto[:250] + "..."
			}
			hist.WriteString(fmt.Sprintf("%s: %s\n", role, texto))
		}
		hist.WriteString("\n")
	}

	var tom string
	if modo == "profissional" {
		tom = `TOM: Profissional, técnico-jurídico, dirige-te a um colega do foro.`
	} else {
		tom = `TOM: Acessível mas profissional. Explica termos técnicos.`
	}

	prompt := fmt.Sprintf(`És o Assistente Jurídico do Base Legal, especializado em Direito moçambicano.

%sO utilizador colocou uma questão jurídica. A nossa base de dados interna NÃO contém o diploma específico mencionado.

%sPERGUNTA: "%s"

═══════════════════════════════════════════════════════════
REGRA CRÍTICA — HONESTIDADE INTELECTUAL
═══════════════════════════════════════════════════════════

Tu és um assistente JURÍDICO, não um enciclopedista. O teu papel é:

✅ DAR O QUE SABES COM CERTEZA — conceitos jurídicos universais, princípios gerais, enquadramento constitucional
❌ NÃO INVENTAR — números de artigos, números de lei, datas específicas de Moçambique que não tenhas na base
⚠️  SER HONESTO — se não tens certeza, diz "não tenho informação verificada sobre isto"

═══════════════════════════════════════════════════════════
COMO RESPONDER
═══════════════════════════════════════════════════════════

1. Se for um CONCEITO JURÍDICO UNIVERSAL (habeas corpus, prescrição, contrato, usucapião):
   → Explica o conceito de forma clara
   → Menciona o enquadramento geral
   → Dá exemplos
   → Confiança: ALTA

2. Se for uma LEI ESPECÍFICA de Moçambique que NÃO tens:
   → DIZ CLARAMENTE: "Não tenho o texto específico desta lei na minha base de dados"
   → NÃO afirmes o que a lei diz
   → Podes dar enquadramento geral (ex: "esta matéria enquadra-se no âmbito de...")
   → NÃO cites números de artigos
   → Confiança: BAIXA

3. Se for sobre OUTRO PAÍS:
   → Diz: "A minha especialização é Direito moçambicano. Em termos comparativos..."
   → Confiança: MÉDIA

4. Se NÃO for Direito:
   → Responde: "Sou especializado em Direito moçambicano. Posso ajudar-te com questões jurídicas."
   → Não dês informação fora do Direito.

═══════════════════════════════════════════════════════════
FRASES A USAR (cautela)
═══════════════════════════════════════════════════════════

✅ "Não tenho o texto específico desta lei na minha base de dados."
✅ "Em termos gerais, esta matéria enquadra-se no âmbito de..."
✅ "Para a redacção exacta da norma, é necessário consultar o Boletim da República."
✅ "Recomendo a consulta de um advogado para análise do caso concreto."
✅ "A minha base de dados é centrada na legislação moçambicana indexada."

❌ NÃO DIGAS: "A lei moçambicana diz que..." (se não tens a lei)
❌ NÃO DIGAS: "O Artigo X estabelece..." (se não tens a certeza)
❌ NÃO DIGAS: "Segundo a Lei Y/ZZZZ..." (se não tens a lei)

═══════════════════════════════════════════════════════════

FORMATO (APENAS JSON):
{
  "tipo": "conceito_universal|lei_especifica|outro_pais|fora_direito|desconhecido",
  "resposta": "resposta em markdown"
}

Responde APENAS o JSON:`, tom, hist.String(), pergunta)

	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resposta, err := ChamarGemini(ctxTimeout, s.apiKey, prompt)
	if err != nil {
		return nil, err
	}

	// Limpar markdown
	resposta = strings.TrimSpace(resposta)
	resposta = strings.TrimPrefix(resposta, "```json")
	resposta = strings.TrimPrefix(resposta, "```")
	resposta = strings.TrimSuffix(resposta, "```")
	resposta = strings.TrimSpace(resposta)

	// Parse JSON robusto
	var parsed struct {
		Tipo     string `json:"tipo"`
		Resposta string `json:"resposta"`
	}

	if err := json.Unmarshal([]byte(resposta), &parsed); err != nil {
		log.Printf("⚠️  JSON inválido no fallback: %v", err)
		parsed.Resposta = extrairResposta(resposta)
		parsed.Tipo = "desconhecido"
	}

	tipo := TipoFallback(parsed.Tipo)
	if tipo == "" {
		tipo = FallbackDesconhecido
	}

	texto := strings.TrimSpace(parsed.Resposta)
	if texto == "" {
		texto = "Não consegui processar a tua pergunta. Podes reformular?"
	}

	texto = limparAvisosDuplicados(texto)

	log.Printf("🎓 Fallback: tipo=%s, len=%d", tipo, len(texto))

	return &RespostaFallback{
		Tipo:      tipo,
		Resposta:  texto,
		Confianca: 0.8,
	}, nil
}

// extrairResposta tenta extrair a resposta de um JSON malformado
func extrairResposta(s string) string {
	idx := strings.Index(s, `"resposta"`)
	if idx < 0 {
		return s
	}
	sub := s[idx+len(`"resposta"`):]
	start := strings.Index(sub, `"`)
	if start < 0 {
		return s
	}
	sub = sub[start+1:]
	end1 := strings.Index(sub, `",`)
	end2 := strings.Index(sub, `"}`)
	end := -1
	if end1 >= 0 && (end2 < 0 || end1 < end2) {
		end = end1
	} else {
		end = end2
	}
	if end < 0 {
		return s
	}
	texto := sub[:end]
	texto = strings.ReplaceAll(texto, `\n`, "\n")
	texto = strings.ReplaceAll(texto, `\"`, `"`)
	texto = strings.ReplaceAll(texto, `\\`, `\`)
	return strings.TrimSpace(texto)
}

// limparAvisosDuplicados remove avisos repetidos que o Gemini insere
func limparAvisosDuplicados(texto string) string {
	linhas := strings.Split(texto, "\n")
	var resultado []string
	for _, l := range linhas {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "⚠️ importante") && strings.Contains(lower, "não tenho a lei") {
			continue
		}
		if strings.Contains(lower, "aviso:") && strings.Contains(lower, "consulta a um advogado") {
			continue
		}
		resultado = append(resultado, l)
	}
	return strings.TrimSpace(strings.Join(resultado, "\n"))
}
