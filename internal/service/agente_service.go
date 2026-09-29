package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	Pergunta string `json:"pergunta"`
	Modo     string `json:"modo"`
	Email    string `json:"email"`
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

	// ============================================
	// 1. EXIGIR UTILIZADOR INSCRITO
	// ============================================
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

	// ============================================
	// 2. VERIFICAR LIMITE (só modo profissional)
	// ============================================
	if req.Modo == "profissional" && s.limites != nil {
		pode, contagem, limite, err := s.limites.PodePerguntar(req.Email, req.Modo)
		if err != nil {
			fmt.Printf("⚠️  Erro a verificar limite: %v\n", err)
		} else if !pode {
			return &AgenteResponse{
				LimiteAtingido:     true,
				PerguntasRestantes: 0,
				Mensagem: fmt.Sprintf(
					"Atingiste o limite de %d perguntas gratuitas no Modo Profissional. Faz upgrade para continuar.",
					limite,
				),
				Modo:      req.Modo,
				Timestamp: time.Now(),
			}, nil
		} else {
			fmt.Printf("📊 Limite: %d/%d usadas (%s)\n", contagem, limite, req.Email)
		}
	}

	// ============================================
	// 3. CARREGAR BD
	// ============================================
	inicio := time.Now()
	todosArtigos, err := s.buscador.ListarTodos()
	if err != nil {
		return nil, fmt.Errorf("erro ao carregar artigos: %v", err)
	}
	fmt.Printf("📚 Carregados %d artigos em %v\n", len(todosArtigos), time.Since(inicio))

	if len(todosArtigos) == 0 {
		return &AgenteResponse{
			Resposta:  "A base de dados está vazia.",
			Artigos:   []domain.Artigo{},
			Modo:      req.Modo,
			Timestamp: time.Now(),
		}, nil
	}

	artigosRelevantes := s.rankearArtigos(req.Pergunta, todosArtigos, 8)

	// ============================================
	// 4. SEM API KEY
	// ============================================
	if s.apiKey == "" {
		return &AgenteResponse{
			Resposta:  fmt.Sprintf("Analisei os %d artigos. Aqui estão os mais relevantes:", len(todosArtigos)),
			Artigos:   artigosRelevantes,
			Modo:      req.Modo,
			Total:     len(todosArtigos),
			Timestamp: time.Now(),
		}, nil
	}

	// ============================================
	// 5. GEMINI
	// ============================================
	resposta, err := s.gerarResposta(ctx, req.Pergunta, req.Modo, todosArtigos, artigosRelevantes)
	if err != nil {
		fmt.Printf("⚠️  Erro Gemini: %v\n", err)
		return &AgenteResponse{
			Resposta:  fmt.Sprintf("Analisei os %d artigos. Aqui estão os mais relevantes:", len(todosArtigos)),
			Artigos:   artigosRelevantes,
			Modo:      req.Modo,
			Total:     len(todosArtigos),
			Timestamp: time.Now(),
		}, nil
	}

	// Registar uso (só profissional)
	if req.Modo == "profissional" && s.limites != nil {
		_ = s.limites.RegistarUso(req.Email, req.Modo)
	}

	// Calcular restantes
	restantes := -1
	if req.Modo == "profissional" && s.limites != nil {
		uso, err := s.limites.ObterUso(req.Email)
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
		Total:              len(todosArtigos),
		Timestamp:          time.Now(),
		PerguntasRestantes: restantes,
	}, nil
}

// ============================================
// RANKING DE RELEVÂNCIA
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
	todosArtigos []domain.Artigo,
	artigosRelevantes []domain.Artigo,
) (string, error) {

	var stats strings.Builder
	stats.WriteString(fmt.Sprintf("Total de artigos na base de dados: %d\n", len(todosArtigos)))

	leis := make(map[string]bool)
	categorias := make(map[string]int)
	for _, a := range todosArtigos {
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

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 2048,
			"topP":            0.95,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini erro %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia do Gemini")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
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

func (s *AgenteService) gerarResposta(
	ctx context.Context,
	pergunta string,
	modo string,
	todosArtigos []domain.Artigo,
	artigosRelevantes []domain.Artigo,
) (string, error) {

	var stats strings.Builder
	stats.WriteString(fmt.Sprintf("Total de artigos na base de dados: %d\n", len(todosArtigos)))

	leis := make(map[string]bool)
	categorias := make(map[string]int)
	for _, a := range todosArtigos {
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

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 2048,
			"topP":            0.95,
		},
	}

	jsonData, _ := json.Marshal(payload)

	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent"

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini erro %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	json.Unmarshal(body, &result)

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("resposta vazia")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
