package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"time"
)

// ============================================
// STREAMING SSE
// ============================================

// EventoStream representa um evento enviado ao cliente
type EventoStream struct {
	Tipo      string      `json:"tipo"`      // "progresso", "token", "artigos", "fim", "erro"
	Dados     interface{} `json:"dados"`     // dados do evento
	Timestamp time.Time   `json:"timestamp"`
}

// StreamCallback é chamado para cada evento
type StreamCallback func(evento EventoStream)

// ProcessarComStream processa uma pergunta e envia eventos em tempo real
func (s *AgenteService) ProcessarComStream(
	ctx context.Context,
	req AgenteRequest,
	callback StreamCallback,
) error {

	// ============================================
	// 1. VALIDAÇÃO INICIAL
	// ============================================
	callback(EventoStream{
		Tipo:      "progresso",
		Dados:     map[string]string{"mensagem": "🔍 A validar pedido..."},
		Timestamp: time.Now(),
	})

	if req.Pergunta == "" {
		callback(EventoStream{
			Tipo:      "erro",
			Dados:     map[string]string{"mensagem": "pergunta vazia"},
			Timestamp: time.Now(),
		})
		return fmt.Errorf("pergunta vazia")
	}

	if req.Modo == "" {
		req.Modo = "cidadao"
	}

	// ============================================
	// 2. VALIDAR UTILIZADOR
	// ============================================
	if req.Email == "" {
		callback(EventoStream{
			Tipo: "precisa_login",
			Dados: map[string]string{
				"mensagem": "Precisas de criar conta para usar o Assistente IA.",
			},
			Timestamp: time.Now(),
		})
		return fmt.Errorf("login necessário")
	}

	if s.validador != nil {
		existe, err := s.validador.UtilizadorExiste(req.Email)
		if err != nil || !existe {
			callback(EventoStream{
				Tipo:      "precisa_login",
				Dados:     map[string]string{"mensagem": "A tua conta não foi encontrada."},
				Timestamp: time.Now(),
			})
			return fmt.Errorf("utilizador não encontrado")
		}
	}

	// ============================================
	// 3. VERIFICAR LIMITE
	// ============================================
	if s.limites != nil {
		pode, contagem, limite, err := s.limites.PodePerguntar(req.Email, req.Modo)
		if err != nil {
			log.Printf("⚠️  Erro a verificar limite: %v", err)
		} else if !pode {
			callback(EventoStream{
				Tipo: "limite_atingido",
				Dados: map[string]interface{}{
					"mensagem": fmt.Sprintf("Atingiste o limite de %d perguntas.", limite),
					"limite":   limite,
				},
				Timestamp: time.Now(),
			})
			return fmt.Errorf("limite atingido")
		} else {
			log.Printf("📊 [%s] Limite: %d/%d usadas", req.Modo, contagem, limite)
		}
	}

	// ============================================
	// 4. RACIOCÍNIO COM IA (classificar intenção)
	// ============================================
	callback(EventoStream{
		Tipo:      "progresso",
		Dados:     map[string]string{"mensagem": "🧠 A analisar a tua mensagem..."},
		Timestamp: time.Now(),
	})

	intencao, errRac := s.AnalisarIntencao(ctx, req.Pergunta, req.Historico)

	// Se for resposta directa (saudação, anúncio, fora do escopo, etc.)
	if errRac == nil && intencao != nil && ehRespostaDirecta(intencao.Tipo) {
		resposta := intencao.RespostaDirecta
		if resposta == "" {
			resposta = gerarRespostaDirecta(intencao)
		}

		callback(EventoStream{
			Tipo: "fim",
			Dados: map[string]interface{}{
				"resposta":            resposta,
				"artigos":             []interface{}{},
				"total_artigos":       0,
				"perguntas_restantes": -1,
			},
			Timestamp: time.Now(),
		})
		return nil
	}

	// ============================================
	// 5. BUSCA FTS
	// ============================================
	callback(EventoStream{
		Tipo:      "progresso",
		Dados:     map[string]string{"mensagem": "🔍 A procurar na legislação..."},
		Timestamp: time.Now(),
	})

	inicioBusca := time.Now()

	queryBusca := ""
	if intencao != nil && intencao.QueryBusca != "" {
		queryBusca = intencao.QueryBusca
	}

	if queryBusca == "" {
		palavras := extrairPalavrasChave(req.Pergunta)
		queryBusca = strings.Join(palavras, " ")
		if queryBusca == "" {
			queryBusca = req.Pergunta
		}
	}

	log.Printf("🔍 Query final: %s", queryBusca)

	candidatos, err := s.buscador.BuscarFullText(queryBusca, 50)
	if err != nil || len(candidatos) == 0 {
		candidatos, err = s.buscador.BuscarPorTexto(queryBusca)
		if err != nil {
			callback(EventoStream{
				Tipo:      "erro",
				Dados:     map[string]string{"mensagem": "erro na busca"},
				Timestamp: time.Now(),
			})
			return err
		}
	}

	duracaoBusca := time.Since(inicioBusca)
	log.Printf("🔍 FTS: %d candidatos em %v", len(candidatos), duracaoBusca)

	if len(candidatos) == 0 {
		// ============================================
		// FALLBACK: Cultura Geral Jurídica
		// ============================================
		callback(EventoStream{
			Tipo:      "progresso",
			Dados:     map[string]string{"mensagem": "🎓 A responder com conhecimento geral..."},
			Timestamp: time.Now(),
		})

		fallback, errFb := s.ResponderComCulturaGeral(ctx, req.Pergunta, req.Modo, req.Historico)
		if errFb != nil {
			log.Printf("⚠️  Fallback falhou: %v", errFb)
			callback(EventoStream{
				Tipo: "progresso",
				Dados: map[string]string{
					"mensagem": "😕 Nenhum artigo relevante encontrado",
				},
				Timestamp: time.Now(),
			})
			return nil
		}

		// Registar uso
		if s.limites != nil {
			_ = s.limites.RegistarUso(req.Email, req.Modo)
		}

		callback(EventoStream{
			Tipo: "fim",
			Dados: map[string]interface{}{
				"resposta":            fallback.Resposta + fallback.AvisoLegal,
				"artigos":             []interface{}{},
				"total_artigos":       0,
				"perguntas_restantes": -1,
				"fonte":               "conhecimento_geral",
				"tipo_fallback":       string(fallback.Tipo),
			},
			Timestamp: time.Now(),
		})
		return nil
	}

	callback(EventoStream{
		Tipo: "progresso",
		Dados: map[string]interface{}{
			"mensagem": fmt.Sprintf("📚 %d artigos encontrados", len(candidatos)),
			"total":    len(candidatos),
		},
		Timestamp: time.Now(),
	})

	// ============================================
	// 6. RANKING
	// ============================================
	callback(EventoStream{
		Tipo:      "progresso",
		Dados:     map[string]string{"mensagem": "⚖️ A analisar a legislação..."},
		Timestamp: time.Now(),
	})

	artigosRelevantes := s.rankearArtigos(req.Pergunta, candidatos, 8)

	totalArtigos, _ := s.buscador.Contar()
	if totalArtigos == 0 {
		totalArtigos = len(candidatos)
	}

	// ============================================
	// 7. GEMINI
	// ============================================
	callback(EventoStream{
		Tipo:      "progresso",
		Dados:     map[string]string{"mensagem": "✍️ A redigir resposta..."},
		Timestamp: time.Now(),
	})

	if s.apiKey == "" {
		callback(EventoStream{
			Tipo: "fim",
			Dados: map[string]interface{}{
				"resposta":            fmt.Sprintf("Analisei %d artigos. Aqui estão os mais relevantes:", totalArtigos),
				"artigos":             artigosRelevantes,
				"total_artigos":       totalArtigos,
				"perguntas_restantes": -1,
			},
			Timestamp: time.Now(),
		})
		return nil
	}

	resposta, err := s.gerarRespostaStream(ctx, req.Pergunta, req.Modo, totalArtigos, candidatos, artigosRelevantes, req.Historico, callback)
	if err != nil {
		log.Printf("⚠️  Erro Gemini: %v", err)
		callback(EventoStream{
			Tipo: "fim",
			Dados: map[string]interface{}{
				"resposta":            fmt.Sprintf("Analisei %d artigos. Aqui estão os mais relevantes:", totalArtigos),
				"artigos":             artigosRelevantes,
				"total_artigos":       totalArtigos,
				"perguntas_restantes": -1,
			},
			Timestamp: time.Now(),
		})
		return nil
	}

	// ============================================
	// 8. REGISTAR USO
	// ============================================
	if s.limites != nil {
		_ = s.limites.RegistarUso(req.Email, req.Modo)
	}

	restantes := -1
	if s.limites != nil {
		var uso map[string]interface{}
		if req.Modo == "cidadao" {
			uso, _ = s.limites.ObterUsoCidadao(req.Email)
		} else {
			uso, _ = s.limites.ObterUso(req.Email)
		}
		if r, ok := uso["restante"].(int); ok {
			restantes = r
		}
	}

	// ============================================
	// 9. EVENTO FINAL
	// ============================================
	callback(EventoStream{
		Tipo: "fim",
		Dados: map[string]interface{}{
			"resposta":            resposta,
			"artigos":             artigosRelevantes,
			"total_artigos":       totalArtigos,
			"perguntas_restantes": restantes,
		},
		Timestamp: time.Now(),
	})

	return nil
}


// gerarRespostaStream chama o Gemini e envia tokens em tempo real
func (s *AgenteService) gerarRespostaStream(
	ctx context.Context,
	pergunta string,
	modo string,
	totalArtigos int,
	candidatos []domain.Artigo,
	artigosRelevantes []domain.Artigo,
	historico []MensagemChat,
	callback StreamCallback,
) (string, error) {

	// Construir prompt (mesmo do gerarResposta)
	prompt := s.construirPrompt(pergunta, modo, totalArtigos, candidatos, artigosRelevantes, historico)

	// Por agora, chamar o Gemini normal e simular streaming
	// (o streaming real precisa de SSE do Gemini, que é mais complexo)
	resposta, err := ChamarGemini(ctx, s.apiKey, prompt)
	if err != nil {
		return "", err
	}

	// Simular streaming: dividir a resposta em pedaços
	palavras := strings.Fields(resposta)
	var buffer strings.Builder

	for i, palavra := range palavras {
		buffer.WriteString(palavra)
		buffer.WriteString(" ")

		// Enviar a cada 5 palavras (para não sobrecarregar)
		if i%5 == 0 || i == len(palavras)-1 {
			callback(EventoStream{
				Tipo: "token",
				Dados: map[string]string{
					"texto": buffer.String(),
				},
				Timestamp: time.Now(),
			})
			buffer.Reset()
			time.Sleep(50 * time.Millisecond) // pausa para efeito visual
		}
	}

	return resposta, nil
}


// ehPerguntaGenerica detecta perguntas que pedem continuação/análise
// sem conteúdo jurídico próprio
func ehPerguntaGenerica(pergunta string) bool {
	p := strings.ToLower(pergunta)

	genericas := []string{
		// Aprofundar
		"aprofunda", "aprofundar", "aprofundamento",
		"profund", "detalha", "detalhar",
		"desenvolve", "desenvolver", "elabora", "elaborar",
		"mais sobre", "mais detalhe", "mais detalhes",
		"fala-me mais", "diz-me mais", "conta-me mais",

		// Continuar
		"continua", "continuar", "continue", "prossegue",

		// Resumir
		"resume", "resumo", "resumir", "sintetiza", "sintese", "sumario",

		// Correlacionar
		"correlaciona", "correlacionar", "relaciona", "relacionar",
		"liga", "ligar", "conecta", "conectar",

		// Explicar de novo
		"explica melhor", "explica mais", "explica novamente", "explica de novo",

		// Pedir análise
		"analisa", "analisar", "analise", "análise",
		"estuda", "estudar", "estudo",
		"avalia", "avaliar", "avaliação",
		"interpreta", "interpretar", "interpretação",

		// Exemplos
		"exemplo", "exemplos", "exemplifica", "ilustra", "caso pratico",

		// Referência ao anterior
		"isso", "aquilo", "aquele", "aquela", "estes", "estas",
		"o assunto", "esse tema", "esse assunto", "este tema",
		"do que falaste", "do que disseste", "que mencionaste",
		"que citaste", "que referiste",

		// Conectores
		"e sobre", "e quanto a", "e em relacao",
		"agora", "entao", "portanto",

		// Verbos genéricos
		"fazer", "faz", "feito", "podes", "podias", "consegues",
	}

	for _, g := range genericas {
		if strings.Contains(p, g) {
			return true
		}
	}

	return false
}


