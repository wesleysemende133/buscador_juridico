package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

const (
	DEFAULT_MODEL  = "gemini-flash-latest"
	MAX_CHUNK_SIZE = 8000
	MAX_RETRIES    = 5
	RETRY_DELAY    = 8 * time.Second
)

// ============================================
// CLIENTE GEMINI
// ============================================

type GeminiExtractor struct {
	client *genai.Client
	model  string
}

type ArtigoExtraido struct {
	Numero        string   `json:"numero"`
	Texto         string   `json:"texto"`
	Lei           string   `json:"lei"`
	LeiNumero     string   `json:"lei_numero"`
	DataVigencia  string   `json:"data_vigencia"`
	PalavrasChave []string `json:"palavras_chave"`
}

type ExtracaoResultado struct {
	Artigos []ArtigoExtraido `json:"artigos"`
}

// ============================================
// NOVO CLIENTE GEMINI
// ============================================

func NewGeminiExtractor(ctx context.Context, apiKey string) (*GeminiExtractor, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %v", err)
	}

	return &GeminiExtractor{
		client: client,
		model:  DEFAULT_MODEL,
	}, nil
}

func (g *GeminiExtractor) SetModel(model string) {
	g.model = model
}

// ============================================
// EXTRAIR ARTIGOS COM CHUNKING
// ============================================

func (g *GeminiExtractor) ExtrairArtigos(ctx context.Context, texto string, fonte string) ([]domain.Artigo, error) {
	log.Printf("🧠 Extraindo artigos com Gemini de: %s (tamanho: %d)", fonte, len(texto))

	var todosArtigos []domain.Artigo

	chunks := g.splitText(texto, MAX_CHUNK_SIZE)
	log.Printf("📦 Texto dividido em %d chunks", len(chunks))

	for i, chunk := range chunks {
		log.Printf("📦 Processando chunk %d/%d", i+1, len(chunks))

		prompt := g.construirPrompt(chunk, fonte)
		resposta, err := g.chamarIAComRetry(ctx, prompt, i+1)
		if err != nil {
			log.Printf("⚠️ Erro no chunk %d (após retries): %v", i+1, err)
			continue
		}

		respostaLimpa := g.limparResposta(resposta)
		log.Printf("🔍 Resposta bruta do chunk %d (primeiros 500 chars): %.500s", i+1, respostaLimpa)

		artigosExtraidos, err := g.parsearResposta(respostaLimpa)
		if err != nil {
			log.Printf("⚠️ Erro ao parsear chunk %d: %v", i+1, err)
			continue
		}

		for _, item := range artigosExtraidos {
			artigo := g.converterParaArtigo(item, fonte)
			todosArtigos = append(todosArtigos, artigo)
		}

		log.Printf("✅ Chunk %d extraiu %d artigos", i+1, len(artigosExtraidos))

		// Pausa entre chunks para evitar rate limit
		time.Sleep(3 * time.Second)
	}

	log.Printf("✅ Gemini extraiu %d artigos de %s", len(todosArtigos), fonte)
	return todosArtigos, nil
}

// ============================================
// PARSEAR RESPOSTA (SUPORTA MÚLTIPLOS FORMATOS)
// ============================================

func (g *GeminiExtractor) parsearResposta(resposta string) ([]ArtigoExtraido, error) {
	resposta = strings.TrimSpace(resposta)

	// Se estiver vazio, retorna vazio
	if resposta == "" || resposta == "{}" || resposta == "[]" {
		return []ArtigoExtraido{}, nil
	}

	// Tentativa 1: Formato esperado {"artigos": [...]}
	var resultado ExtracaoResultado
	if err := json.Unmarshal([]byte(resposta), &resultado); err == nil && len(resultado.Artigos) > 0 {
		return resultado.Artigos, nil
	}

	// Tentativa 2: Formato alternativo [...] (array direto)
	var artigos []ArtigoExtraido
	if err := json.Unmarshal([]byte(resposta), &artigos); err == nil && len(artigos) > 0 {
		return artigos, nil
	}

	// Tentativa 3: Formato com wrapper mas a chave é diferente
	var generico map[string]interface{}
	if err := json.Unmarshal([]byte(resposta), &generico); err == nil {
		for _, v := range generico {
			if arr, ok := v.([]interface{}); ok {
				var items []ArtigoExtraido
				for _, item := range arr {
					b, _ := json.Marshal(item)
					var a ArtigoExtraido
					if json.Unmarshal(b, &a) == nil && a.Texto != "" {
						items = append(items, a)
					}
				}
				if len(items) > 0 {
					return items, nil
				}
			}
		}
	}

	// Tentativa 4: Procurar JSON dentro do texto (caso a IA adicione texto extra)
	inicio := strings.Index(resposta, "{")
	fim := strings.LastIndex(resposta, "}")
	if inicio >= 0 && fim > inicio {
		jsonStr := resposta[inicio : fim+1]
		var resultado2 ExtracaoResultado
		if err := json.Unmarshal([]byte(jsonStr), &resultado2); err == nil && len(resultado2.Artigos) > 0 {
			return resultado2.Artigos, nil
		}
	}

	return []ArtigoExtraido{}, nil
}

// ============================================
// CHAMAR IA COM RETRY
// ============================================

func (g *GeminiExtractor) chamarIAComRetry(ctx context.Context, prompt string, chunkNum int) (string, error) {
	var ultimoErro error

	for tentativa := 1; tentativa <= MAX_RETRIES; tentativa++ {
		resposta, err := g.chamarIA(ctx, prompt)
		if err == nil {
			return resposta, nil
		}

		ultimoErro = err

		// Se for erro 503 (sobrecarga)
		if strings.Contains(err.Error(), "503") {
			espera := time.Duration(tentativa) * RETRY_DELAY
			log.Printf("⏳ Chunk %d: tentativa %d/%d falhou (503). A aguardar %v...",
				chunkNum, tentativa, MAX_RETRIES, espera)
			time.Sleep(espera)
			continue
		}

		// Se for erro 429 (rate limit)
		if strings.Contains(err.Error(), "429") {
			espera := time.Duration(tentativa*2) * RETRY_DELAY
			log.Printf("⏳ Chunk %d: tentativa %d/%d falhou (429). A aguardar %v...",
				chunkNum, tentativa, MAX_RETRIES, espera)
			time.Sleep(espera)
			continue
		}

		// Outros erros
		log.Printf("⏳ Chunk %d: tentativa %d/%d falhou. A aguardar %v...",
			chunkNum, tentativa, MAX_RETRIES, RETRY_DELAY)
		time.Sleep(RETRY_DELAY)
	}

	return "", fmt.Errorf("falhou após %d tentativas: %v", MAX_RETRIES, ultimoErro)
}

// ============================================
// DIVIDIR TEXTO EM CHUNKS
// ============================================

func (g *GeminiExtractor) splitText(texto string, maxSize int) []string {
	var chunks []string
	texto = strings.TrimSpace(texto)

	if len(texto) <= maxSize {
		return []string{texto}
	}

	linhas := strings.Split(texto, "\n")
	var chunk strings.Builder

	for _, linha := range linhas {
		if chunk.Len()+len(linha)+1 > maxSize {
			chunks = append(chunks, chunk.String())
			chunk.Reset()
		}
		chunk.WriteString(linha + "\n")
	}

	if chunk.Len() > 0 {
		chunks = append(chunks, chunk.String())
	}

	return chunks
}

// ============================================
// CONSTRUIR PROMPT
// ============================================

func (g *GeminiExtractor) construirPrompt(texto, fonte string) string {
	return fmt.Sprintf(`
Você é um especialista em extração de dados jurídicos de Moçambique.

Analise o texto abaixo e extraia TODOS os artigos de lei que encontrar.

REGRAS:
1. Identifique artigos no formato "Art. X", "Artigo X", "Art. Xº"
2. Para cada artigo, extraia:
   - numero: o número do artigo (só o número, ex: "1", "2", "1.583")
   - texto: o texto completo do artigo
   - lei: o nome da lei
   - lei_numero: o número da lei (se disponível)
   - data_vigencia: data de vigência (formato YYYY-MM-DD)
   - palavras_chave: 5-10 palavras-chave
3. Ignore cabeçalhos, rodapés, ementas e assinaturas
4. Se não encontrar artigos, retorne {"artigos": []}

IMPORTANTE: Retorne SEMPRE um objeto JSON no formato:
{"artigos": [{"numero": "...", "texto": "...", ...}]}

NUNCA retorne um array direto. SEMPRE use o wrapper "artigos".

FONTE: %s

TEXTO:
%s

JSON:`, fonte, texto)
}

// ============================================
// CHAMAR GEMINI
// ============================================

func (g *GeminiExtractor) chamarIA(ctx context.Context, prompt string) (string, error) {
	temperatura := float32(0.1)
	config := &genai.GenerateContentConfig{
		Temperature:      &temperatura,
		ResponseMIMEType: "application/json",
	}

	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}

	resp, err := g.client.Models.GenerateContent(
		ctx,
		g.model,
		contents,
		config,
	)

	if err != nil {
		return "", fmt.Errorf("erro ao chamar Gemini: %v", err)
	}

	return resp.Text(), nil
}

// ============================================
// LIMPAR RESPOSTA
// ============================================

func (g *GeminiExtractor) limparResposta(resposta string) string {
	resposta = strings.TrimSpace(resposta)
	resposta = strings.TrimPrefix(resposta, "```json")
	resposta = strings.TrimPrefix(resposta, "```")
	resposta = strings.TrimSuffix(resposta, "```")
	return strings.TrimSpace(resposta)
}

// ============================================
// CONVERTER PARA DOMAIN.ARTIGO
// ============================================

func (g *GeminiExtractor) converterParaArtigo(item ArtigoExtraido, fonte string) domain.Artigo {
	var dataVigencia time.Time
	if item.DataVigencia != "" {
		if t, err := time.Parse("2006-01-02", item.DataVigencia); err == nil {
			dataVigencia = t
		}
	}
	if dataVigencia.IsZero() {
		dataVigencia = time.Now()
	}

	nomeLei := strings.ReplaceAll(strings.ToLower(item.Lei), " ", "_")
	id := fmt.Sprintf("%s_art_%s", nomeLei, item.Numero)
	id = strings.ReplaceAll(id, ".", "_")

	return domain.Artigo{
		ID:             id,
		Lei:            item.Lei,
		LeiNumero:     item.LeiNumero,
		Artigo:         fmt.Sprintf("Art. %s", item.Numero),
		Texto:          item.Texto,
		PalavrasChave:  item.PalavrasChave,
		Versao:         1,
		DataVigencia:   dataVigencia,
		DataPublicacao: time.Now(),
		Status:         "Vigente",
		Fonte:          fonte,
		CriadoEm:       time.Now(),
		AtualizadoEm:   time.Now(),
		AprovadoPor:    "Gemini IA",
	}
}
