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
// RACIOCÍNIO INTELIGENTE COM IA
// ============================================

// Intencao representa o que o utilizador quer
type Intencao struct {
	Tipo             string `json:"tipo"`
	QueryBusca       string `json:"query_busca"`
	UsarHistorico    bool   `json:"usar_historico"`
	TemaJuridico     string `json:"tema_juridico"`
	RespostaDirecta  string `json:"resposta_directa,omitempty"`
	AlertaAdvogado   bool   `json:"alerta_advogado"`
	MotivoAlerta     string `json:"motivo_alerta,omitempty"`
	InstrucaoExtra   string `json:"instrucao_extra,omitempty"`
	Explicacao       string `json:"explicacao"`
}

// AnalisarIntencao usa o Gemini para classificar a intenção do utilizador
func (s *AgenteService) AnalisarIntencao(
	ctx context.Context,
	pergunta string,
	historico []MensagemChat,
) (*Intencao, error) {

	// Construir histórico (só mensagens, sem artigos)
	var hist strings.Builder
	if len(historico) > 0 {
		hist.WriteString("HISTÓRICO DA CONVERSA:\n")
		for _, m := range historico {
			role := "User"
			if m.Role == "assistant" {
				role = "Bot"
			}
			texto := m.Texto
			if len(texto) > 300 {
				texto = texto[:300] + "..."
			}
			hist.WriteString(fmt.Sprintf("%s: %s\n", role, texto))
		}
		hist.WriteString("\n")
	}

	prompt := fmt.Sprintf(`És um CLASSIFICADOR DE INTENÇÕES para um assistente jurídico moçambicano.

O assistente só responde sobre DIREITO MOÇAMBICANO (leis, artigos, direitos, deveres, processos judiciais).

%sNOVA MENSAGEM DO UTILIZADOR: "%s"

Analisa a mensagem e devolve APENAS um JSON válido:

{
  "tipo": "<tipo>",
  "query_busca": "<palavras-chave para FTS>",
  "usar_historico": true|false,
  "tema_juridico": "<tema em 1-2 palavras ou vazio>",
  "resposta_directa": "<texto ou vazio>",
  "alerta_advogado": true|false,
  "motivo_alerta": "<razão ou vazio>",
  "instrucao_extra": "<instrução para o gerador de resposta ou vazio>",
  "explicacao": "<breve razão>"
}

TIPOS POSSÍVEIS:
- "saudacao" → cumprimenta ("ola", "bom dia", "tudo bem")
- "anuncio" → diz que vai perguntar mas ainda não perguntou ("tenho uma pergunta", "posso perguntar?")
- "agradecimento" → agradece ("obrigado", "valeu")
- "despedida" → despede-se ("tchau", "até logo")
- "fora_do_escopo" → pergunta sobre algo que NÃO é Direito (ex: "como fazer bolo", "quem ganhou o jogo")
- "nova_pergunta" → pergunta jurídica nova (sem relação com o histórico)
- "aprofundar" → quer mais detalhes sobre o tema anterior
- "resumir" → quer um resumo do que foi discutido
- "correlacionar" → quer ligar/comparar artigos anteriores
- "exemplo" → quer um exemplo prático
- "comparar" → quer comparar dois temas/leis
- "traduzir" → quer tradução (para outra língua ou linguagem simples)
- "reformular" → quer a mesma resposta de forma diferente
- "esclarecer" → tem dúvida sobre algo dito
- "pede_advogado" → pede explicitamente um advogado

REGRAS:
- Se a mensagem é sobre Direito → usar_historico=false (nova pergunta)
- Se é continuação ("mais", "detalha", "explica") → usar_historico=true
- Se é saudação/anúncio/agradecimento/despedida → resposta_directa (sem busca)
- Se é fora_do_escopo → resposta_directa educada a redireccionar para Direito
- alerta_advogado=true quando:
  * O caso é específico/complexo ("o meu patrão fez X", "fui despedido")
  * Envolve valores concretos, datas, contratos específicos
  * É um litígio em curso ou potencial
  * O user pede opinião jurídica vinculativa
- query_busca: 2-5 palavras-chave em português, sem stopwords
- tema_juridico: ex: "trabalho", "maternidade", "contrato", "família"

Responde APENAS o JSON:`, hist.String(), pergunta)

	ctxTimeout, cancel := context.WithTimeout(ctx, 20*time.Second)
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

	var intencao Intencao
	if err := json.Unmarshal([]byte(resposta), &intencao); err != nil {
		log.Printf("⚠️  JSON inválido: %v", err)
		log.Printf("   Resposta: %.200s", resposta)
		return nil, err
	}

	log.Printf("🧠 tipo=%s tema=%s usar_hist=%v alerta=%v query=%q",
		intencao.Tipo, intencao.TemaJuridico,
		intencao.UsarHistorico, intencao.AlertaAdvogado,
		intencao.QueryBusca)

	return &intencao, nil
}

// ehRespostaDirecta verifica se a intenção devolve resposta sem busca
func ehRespostaDirecta(tipo string) bool {
	switch tipo {
	case "saudacao", "anuncio", "agradecimento", "despedida", "fora_do_escopo":
		return true
	}
	return false
}

// gerarRespostaDirecta devolve uma resposta padrão consoante o tipo
func gerarRespostaDirecta(intencao *Intencao) string {
	switch intencao.Tipo {
	case "saudacao":
		return "Olá! 👋 Sou o Assistente Jurídico do Base Legal.\n\nEstou aqui para ajudar com questões sobre a **legislação moçambicana**. Podes perguntar-me sobre:\n\n- **Direitos laborais** (trabalho, contratos, férias)\n- **Direitos da família** (casamento, divórcio, maternidade)\n- **Constituição** e direitos fundamentais\n- **Processos judiciais**\n\nO que gostarias de saber?"

	case "anuncio":
		return "Claro! Estou aqui para ajudar. 😊\n\nColoca a tua pergunta sobre **legislação moçambicana**. Por exemplo:\n\n- *O que diz a lei sobre o trabalho?*\n- *Direitos da maternidade*\n- *O que diz o artigo 11?*\n\nDiz-me o que precisas!"

	case "agradecimento":
		return "De nada! 😊 Estou sempre aqui para ajudar com questões jurídicas.\n\n**Lembrete:** As informações que forneço são de carácter informativo e não substituem o aconselhamento de um advogado. Para casos específicos, recomendo consultar um profissional do Direito."

	case "despedida":
		return "Até logo! 👋 Foi um prazer ajudar.\n\nSe precisares de mais esclarecimentos sobre legislação moçambicana, volta quando quiseres.\n\n**Lembrete:** Para casos concretos, consulta sempre um advogado."

	case "fora_do_escopo":
		return "Sou um assistente especializado em **Direito Moçambicano**. 🏛️\n\nSó posso ajudar com questões sobre:\n- Leis e artigos\n- Direitos e deveres\n- Processos judiciais\n- Constituição\n\nA tua pergunta parece estar fora do meu âmbito. Se tiveres alguma questão jurídica, estou aqui para ajudar!\n\nSe precisares de ajuda específica para o teu caso, recomendo consultar um **advogado**."
	}

	return "Desculpa, não compreendi. Podes reformular a tua pergunta sobre legislação moçambicana?"
}
