package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type AgenteService struct {
	buscador  repository.Buscador
	limites   *LimitesService
	validador *AuthValidator
	apiKey    string
	client    *http.Client
}

func NewAgenteService(
	buscador repository.Buscador,
	limites *LimitesService,
	validador *AuthValidator,
) *AgenteService {
	return &AgenteService{
		buscador:  buscador,
		limites:   limites,
		validador: validador,
		apiKey:    os.Getenv("GEMINI_API_KEY"),
		client:    &http.Client{Timeout: 60 * time.Second},
	}
}

// ============================================
// TIPOS
// ============================================

type AgenteRequest struct {
	Pergunta  string         `json:"pergunta"`
	Modo      string         `json:"modo"`
	Email     string         `json:"email"`
	Historico []MensagemChat `json:"historico,omitempty"`
}

// MensagemChat representa uma mensagem anterior na conversa
type MensagemChat struct {
	Role    string   `json:"role"`              // "user" ou "assistant"
	Texto   string   `json:"texto"`
	Artigos []string `json:"artigos,omitempty"` // IDs dos artigos citados
}

type AgenteResponse struct {
	Resposta           string          `json:"resposta"`
	Artigos            []domain.Artigo `json:"artigos"`
	Modo               string          `json:"modo"`
	Total              int             `json:"total_artigos_analisados"`
	Timestamp          time.Time       `json:"timestamp"`
	LimiteAtingido     bool            `json:"limite_atingido,omitempty"`
	PerguntasRestantes int             `json:"perguntas_restantes,omitempty"`
	Mensagem           string          `json:"mensagem,omitempty"`
	PrecisaLogin       bool            `json:"precisa_login,omitempty"`
}

// ============================================
// PROCESSAR
// ============================================

func (s *AgenteService) Processar(ctx context.Context, req AgenteRequest) (*AgenteResponse, error) {
	if req.Pergunta == "" {
		return nil, fmt.Errorf("pergunta vazia")
	}

	if req.Modo == "" {
		req.Modo = "cidadao"
	}

	// 1. EXIGIR UTILIZADOR INSCRITO
	if req.Email == "" {
		return &AgenteResponse{
			PrecisaLogin: true,
			Mensagem:     "Precisas de criar conta para usar o Assistente IA. A criação é gratuita.",
			Modo:         req.Modo,
			Timestamp:    time.Now(),
		}, nil
	}

	if s.validador != nil {
		existe, err := s.validador.UtilizadorExiste(req.Email)
		if err != nil {
			return nil, fmt.Errorf("erro a validar utilizador: %v", err)
		}
		if !existe {
			return &AgenteResponse{
				PrecisaLogin: true,
				Mensagem:     "A tua conta não foi encontrada. Cria uma conta gratuita para usar o Assistente IA.",
				Modo:         req.Modo,
				Timestamp:    time.Now(),
			}, nil
		}
	}

	// 2. VERIFICAR LIMITE (cidadao 20/dia | profissional 3 total)
	if s.limites != nil {
		pode, contagem, limite, err := s.limites.PodePerguntar(req.Email, req.Modo)
		if err != nil {
			fmt.Printf("⚠️  Erro a verificar limite: %v\n", err)
		} else if !pode {
			mensagem := fmt.Sprintf(
				"Atingiste o limite de %d perguntas. Faz upgrade para continuar.",
				limite,
			)
			if req.Modo == "cidadao" {
				mensagem = fmt.Sprintf(
					"Atingiste o limite de %d perguntas diárias no Modo Cidadão. Volta amanhã ou faz upgrade.",
					limite,
				)
			}
			return &AgenteResponse{
				LimiteAtingido:     true,
				PerguntasRestantes: 0,
				Mensagem:           mensagem,
				Modo:               req.Modo,
				Timestamp:          time.Now(),
			}, nil
		} else {
			fmt.Printf("📊 [%s] Limite: %d/%d usadas (%s)\n", req.Modo, contagem, limite, req.Email)
		}
	}

	// ============================================
	// 3. BUSCA INTELIGENTE (FTS PostgreSQL)
	// Carrega apenas os artigos relevantes
	// ============================================
	inicio := time.Now()

	palavras := extrairPalavrasChave(req.Pergunta)
	queryBusca := strings.Join(palavras, " ")

	if queryBusca == "" {
		queryBusca = req.Pergunta
	}

	// Buscar com Full Text Search (top 50)
	candidatos, err := s.buscador.BuscarFullText(queryBusca, 50)
	if err != nil || len(candidatos) == 0 {
		fmt.Printf("⚠️  FTS falhou ou vazio, usando fallback\n")
		candidatos, err = s.buscador.BuscarPorTexto(queryBusca)
		if err != nil {
			return nil, fmt.Errorf("erro na busca: %v", err)
		}
	}

	fmt.Printf("🔍 FTS: %d candidatos em %v\n", len(candidatos), time.Since(inicio))

	if len(candidatos) == 0 {
		return &AgenteResponse{
			Resposta:  "Não encontrei artigos relevantes para a tua pergunta. Tenta reformular ou usar outras palavras-chave.",
			Artigos:   []domain.Artigo{},
			Modo:      req.Modo,
			Timestamp: time.Now(),
		}, nil
	}

	// Ranking final (top 8)
	artigosRelevantes := s.rankearArtigos(req.Pergunta, candidatos, 8)

	// Total para contexto
	totalArtigos, _ := s.buscador.Contar()
	if totalArtigos == 0 {
		totalArtigos = len(candidatos)
	}

	// 4. SEM API KEY
	if s.apiKey == "" {
		return &AgenteResponse{
			Resposta:  fmt.Sprintf("Analisei %d artigos. Aqui estão os mais relevantes:", totalArtigos),
			Artigos:   artigosRelevantes,
			Modo:      req.Modo,
			Total:     totalArtigos,
			Timestamp: time.Now(),
		}, nil
	}

	// 5. GEMINI
	resposta, err := s.gerarResposta(ctx, req.Pergunta, req.Modo, totalArtigos, candidatos, artigosRelevantes)
	if err != nil {
		fmt.Printf("⚠️  Erro Gemini: %v\n", err)
		return &AgenteResponse{
			Resposta:  fmt.Sprintf("Analisei %d artigos. Aqui estão os mais relevantes:", totalArtigos),
			Artigos:   artigosRelevantes,
			Modo:      req.Modo,
			Total:     totalArtigos,
			Timestamp: time.Now(),
		}, nil
	}

	// Registar uso (ambos os modos)
	if s.limites != nil {
		_ = s.limites.RegistarUso(req.Email, req.Modo)
	}

	// Calcular restantes
	restantes := -1
	if s.limites != nil {
		var uso map[string]interface{}
		var err error
		if req.Modo == "cidadao" {
			uso, err = s.limites.ObterUsoCidadao(req.Email)
		} else {
			uso, err = s.limites.ObterUso(req.Email)
		}
		if err == nil {
			if r, ok := uso["restante"].(int); ok {
				restantes = r
			}
		}
	}

	return &AgenteResponse{
		Resposta:           resposta,
		Artigos:            artigosRelevantes,
		Modo:               req.Modo,
		Total:              totalArtigos,
		Timestamp:          time.Now(),
		PerguntasRestantes: restantes,
	}, nil
}

// ============================================
// RANKING
// ============================================

type artigoScore struct {
	artigo domain.Artigo
	score  int
}

func (s *AgenteService) rankearArtigos(pergunta string, artigos []domain.Artigo, limite int) []domain.Artigo {
	palavras := extrairPalavrasChave(pergunta)

	if len(palavras) == 0 {
		if len(artigos) > limite {
			return artigos[:limite]
		}
		return artigos
	}

	var scored []artigoScore

	for _, a := range artigos {
		score := 0
		textoLower := strings.ToLower(a.Texto)
		leiLower := strings.ToLower(a.Lei)
		artigoLower := strings.ToLower(a.Artigo)
		catLower := strings.ToLower(a.Categoria)
		subcatLower := strings.ToLower(a.Subcategoria)
		palavrasArtigo := strings.ToLower(strings.Join(a.PalavrasChave, " "))

		for _, p := range palavras {
			if strings.Contains(artigoLower, p) {
				score += 10
			}
			if strings.Contains(leiLower, p) {
				score += 5
			}
			if strings.Contains(catLower, p) || strings.Contains(subcatLower, p) {
				score += 4
			}
			if strings.Contains(palavrasArtigo, p) {
				score += 3
			}
			oc := strings.Count(textoLower, p)
			if oc > 0 {
				score += oc
				if oc > 3 {
					score += 2
				}
			}
		}

		if score > 0 {
			scored = append(scored, artigoScore{artigo: a, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	var resultado []domain.Artigo
	for i, s := range scored {
		if i >= limite {
			break
		}
		resultado = append(resultado, s.artigo)
	}

	if len(resultado) == 0 && len(artigos) > 0 {
		lim := limite
		if len(artigos) < lim {
			lim = len(artigos)
		}
		resultado = artigos[:lim]
	}

	return resultado
}

// ============================================
// PALAVRAS-CHAVE
// ============================================

func extrairPalavrasChave(pergunta string) []string {
	stopwords := map[string]bool{
		"o": true, "a": true, "os": true, "as": true, "um": true, "uma": true,
		"de": true, "do": true, "da": true, "dos": true, "das": true,
		"em": true, "no": true, "na": true, "nos": true, "nas": true,
		"por": true, "para": true, "com": true, "sem": true, "sob": true,
		"e": true, "ou": true, "mas": true, "que": true, "qual": true,
		"como": true, "quando": true, "onde": true, "porque": true,
		"é": true, "são": true, "foi": true, "foram": true, "ser": true,
		"ter": true, "tem": true, "têm": true, "há": true, "existe": true,
		"sobre": true, "quais": true, "seu": true, "sua": true,
		"este": true, "esta": true, "esse": true, "essa": true, "isso": true,
		"eu": true, "tu": true, "ele": true, "ela": true, "nós": true,
		"me": true, "te": true, "lhe": true, "vos": true,
		"diz": true, "lei": true, "leis": true,
		"podes": true, "pode": true, "podias": true,
		"detalhar": true, "detalha": true, "detalhes": true,
		"explicar": true, "explica": true,
		"falar": true, "fala": true,
		"querer": true, "quero": true, "queres": true,
		"consegues": true, "consegue": true,
		"gostaria": true, "gostava": true,
		"saber": true, "sei": true, "sabe": true,
		"fazer": true, "faz": true, "feito": true,
		"mais": true, "melhor": true, "menos": true,
		"muito": true, "pouco": true,
		"obrigado": true, "obrigada": true, "por favor": true,
	}

	pergunta = strings.ToLower(pergunta)
	pergunta = strings.ReplaceAll(pergunta, "?", "")
	pergunta = strings.ReplaceAll(pergunta, "!", "")
	pergunta = strings.ReplaceAll(pergunta, ",", " ")
	pergunta = strings.ReplaceAll(pergunta, ".", " ")

	palavras := strings.Fields(pergunta)

	var resultado []string
	for _, p := range palavras {
		if len(p) < 4 {
			continue
		}
		if stopwords[p] {
			continue
		}
		resultado = append(resultado, p)
	}

	return resultado
}

// ============================================
// GEMINI
// ============================================

func (s *AgenteService) gerarResposta(
	ctx context.Context,
	pergunta string,
	modo string,
	totalArtigos int,
	candidatos []domain.Artigo,
	artigosRelevantes []domain.Artigo,
) (string, error) {

	var stats strings.Builder
	stats.WriteString(fmt.Sprintf("Total de artigos na base de dados: %d\n", totalArtigos))
	stats.WriteString(fmt.Sprintf("Artigos analisados nesta busca: %d\n", len(candidatos)))

	leis := make(map[string]bool)
	categorias := make(map[string]int)
	for _, a := range candidatos {
		leis[a.Lei] = true
		categorias[a.Categoria]++
	}

	stats.WriteString(fmt.Sprintf("Total de leis: %d\n", len(leis)))
	stats.WriteString("Categorias disponíveis:\n")
	for cat, count := range categorias {
		stats.WriteString(fmt.Sprintf("  - %s (%d artigos)\n", cat, count))
	}

	var contexto strings.Builder
	contexto.WriteString("\n\nARTIGOS MAIS RELEVANTES:\n\n")

	for i, a := range artigosRelevantes {
		contexto.WriteString(fmt.Sprintf(
			"[Artigo %d]\nLei: %s (%s)\nArtigo: %s\nCategoria: %s\nTexto: %s\n\n---\n\n",
			i+1, a.Lei, a.LeiNumero, a.Artigo, a.Categoria, a.Texto,
		))
	}

	var instrucoes string
	if modo == "profissional" {
		instrucoes = `És o Assistente Jurídico do Base Legal. Falas com profissionais do Direito.

TOM E ESTILO:
- Profissional, mas acessível. Trata por "você".
- Começa com "Com base na análise da legislação moçambicana..."
- Usa terminologia jurídica adequada.
- Cita sempre os artigos específicos.
- NUNCA inventes artigos ou factos.
- NUNCA uses emojis.`
	} else {
		instrucoes = `És o Assistente Jurídico do Base Legal. Ajudas cidadãos comuns.

TOM E ESTILO:
- Amigável, caloroso. Trata por "você".
- Começa com "Boa pergunta! Deixa-me explicar de forma simples."
- Linguagem do dia-a-dia.
- Cita os artigos naturalmente.
- Termina: "Se tiveres um caso específico, o melhor é consultar um advogado."
- NUNCA inventes factos.
- Máximo 250 palavras.`
	}

	prompt := fmt.Sprintf(`%s

CONTEXTO DA BASE DE DADOS:
%s

%s

PERGUNTA: %s

Responde de forma fundamentada. NÃO repitas os artigos no fim — eles são mostrados automaticamente.`,
		instrucoes, stats.String(), contexto.String(), pergunta,
	)

	return ChamarGemini(ctx, s.apiKey, prompt)
}

// construirPrompt cria o prompt para o Gemini
func (s *AgenteService) construirPrompt(
	pergunta string,
	modo string,
	totalArtigos int,
	candidatos []domain.Artigo,
	artigosRelevantes []domain.Artigo,
	historico []MensagemChat,
) string {
	var stats strings.Builder
	stats.WriteString(fmt.Sprintf("Total de artigos na base de dados: %d\n", totalArtigos))
	stats.WriteString(fmt.Sprintf("Artigos analisados nesta busca: %d\n", len(candidatos)))

	var contexto strings.Builder
	contexto.WriteString("\n\nARTIGOS MAIS RELEVANTES:\n\n")

	for i, a := range artigosRelevantes {
		contexto.WriteString(fmt.Sprintf(
			"[Artigo %d]\nLei: %s (%s)\nArtigo: %s\nCategoria: %s\nTexto: %s\n\n---\n\n",
			i+1, a.Lei, a.LeiNumero, a.Artigo, a.Categoria, a.Texto,
		))
	}

	var instrucoes string
	if modo == "profissional" {
		instrucoes = `És o Assistente Jurídico do Base Legal. Falas com profissionais do Direito.

TOM E ESTILO:
- Profissional, mas acessível. Trata por "você".
- Usa terminologia jurídica adequada.
- Cita sempre os artigos específicos.
- NUNCA inventes artigos ou factos.
- Se houver histórico, mantém a continuidade da conversa.`
	} else {
		instrucoes = `És o Assistente Jurídico do Base Legal. Ajudas cidadãos comuns.

TOM E ESTILO:
- Amigável, caloroso. Trata por "você".
- Se for a PRIMEIRA mensagem (sem histórico), começa com "Boa pergunta! Deixa-me explicar de forma simples."
- Se HOUVER histórico, NÃO repitas a saudação — continua naturalmente a conversa.
- Linguagem do dia-a-dia.
- Cita os artigos naturalmente.
- Máximo 250 palavras por resposta.`
	}

	// ============================================
	// HISTÓRICO DA CONVERSA
	// ============================================
	var hist strings.Builder
	if len(historico) > 0 {
		hist.WriteString("\n\nHISTÓRICO DA CONVERSA (usa para contexto, NÃO repitas):\n\n")
		for _, m := range historico {
			role := "Utilizador"
			if m.Role == "assistant" {
				role = "Assistente"
			}
			hist.WriteString(fmt.Sprintf("%s: %s\n\n", role, m.Texto))
		}
		hist.WriteString("\n(Continua a conversa a partir daqui. Se o utilizador pedir para aprofundar, correlacionar ou resumir, refere-te ao que já foi discutido.)\n")
	}

	return fmt.Sprintf(`%s

CONTEXTO DA BASE DE DADOS:
%s

%s
%s

PERGUNTA ACTUAL: %s

Responde de forma fundamentada.`,
		instrucoes, stats.String(), contexto.String(), hist.String(), pergunta,
	)
}
