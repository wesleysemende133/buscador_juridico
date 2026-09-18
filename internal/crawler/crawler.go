package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"               
	"log"
	"net/http"          
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/ledongthuc/pdf"
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

var geminiExtractor *GeminiExtractor
var regexExtractor *RegexExtractor

// Número de workers paralelos
const NUM_WORKERS = 5

// Contador global
var totalPersistidos int
var muPersistidos sync.Mutex

// ============================================
// INICIALIZAR
// ============================================

func InitGeminiExtractor(ctx context.Context, apiKey string) error {
	extractor, err := NewGeminiExtractor(ctx, apiKey)
	if err != nil {
		return err
	}
	geminiExtractor = extractor
	log.Println("🧠 Gemini Extractor inicializado (fallback)")
	return nil
}

func InitRegexExtractor() {
	regexExtractor = NewRegexExtractor()
	log.Println("📖 Regex Extractor inicializado (principal)")
}

func GetTotalPersistidos() int {
	muPersistidos.Lock()
	defer muPersistidos.Unlock()
	return totalPersistidos
}

// ============================================
// BUSCAR LEIS COM WORKERS PARALELOS
// ============================================

func BuscarLeisPlanalto(ctx context.Context, persistir func([]domain.Artigo) (int, error)) ([]domain.Artigo, error) {
	if regexExtractor == nil {
		InitRegexExtractor()
	}

	// Resetar contador
	muPersistidos.Lock()
	totalPersistidos = 0
	muPersistidos.Unlock()

	log.Printf("🕷️  Iniciando crawler com %d workers paralelos...", NUM_WORKERS)

	// ============================================
	// CANAL DE URLS (PDFs a processar)
	// ============================================
	urlsChan := make(chan string, 100)
	
	// Canal para contar artigos extraídos por worker
	resultadosChan := make(chan int, 1000)

	// ============================================
	// INICIAR WORKERS
	// ============================================
	var wg sync.WaitGroup
	ctxWorkers, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	for i := 1; i <= NUM_WORKERS; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker(ctxWorkers, workerID, urlsChan, resultadosChan, persistir)
		}(i)
	}

	// ============================================
	// INICIAR COLETOR DE LINKS
	// ============================================
	go func() {
		defer close(urlsChan)
		coletarLinks(ctx, urlsChan)
	}()

	// ============================================
	// AGUARDAR TODOS OS WORKERS
	// ============================================
	go func() {
		wg.Wait()
		close(resultadosChan)
	}()

	// Contar total
	total := 0
	for count := range resultadosChan {
		total += count
	}

	log.Printf("✅ Total de artigos extraídos: %d | Persistidos: %d", total, GetTotalPersistidos())
	return nil, nil
}

// ============================================
// WORKER (processa PDFs do canal)
// ============================================

func worker(
	ctx context.Context,
	workerID int,
	urlsChan <-chan string,
	resultadosChan chan<- int,
	persistir func([]domain.Artigo) (int, error),
) {
	log.Printf("👷 Worker %d iniciado", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("👷 Worker %d finalizado (contexto cancelado)", workerID)
			return
		case url, ok := <-urlsChan:
			if !ok {
				log.Printf("👷 Worker %d finalizado (canal fechado)", workerID)
				return
			}

			// Processa o PDF
			artigos, err := processarPDF(ctx, url, workerID)
			if err != nil {
				log.Printf("⚠️ Worker %d erro em %s: %v", workerID, url, err)
				resultadosChan <- 0
				continue
			}

			if len(artigos) == 0 {
				resultadosChan <- 0
				continue
			}

			// Persistir imediatamente
			adicionados, err := persistir(artigos)
			if err != nil {
				log.Printf("⚠️ Worker %d erro ao persistir: %v", workerID, err)
				resultadosChan <- 0
				continue
			}

			muPersistidos.Lock()
			totalPersistidos += adicionados
			muPersistidos.Unlock()

			log.Printf("✅ Worker %d: %d extraídos, %d persistidos (total: %d)",
				workerID, len(artigos), adicionados, GetTotalPersistidos())

			resultadosChan <- len(artigos)
		}
	}
}

// ============================================
// PROCESSAR PDF INDIVIDUAL
// ============================================

func processarPDF(ctx context.Context, url string, workerID int) ([]domain.Artigo, error) {
	log.Printf("📄 Worker %d: Baixando %s", workerID, url)

	// Baixar o PDF com timeout
	client := &httpClient{timeout: 60 * time.Second}
	dados, err := client.baixar(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao baixar: %v", err)
	}

	// Extrair texto
	texto, err := extrairTextoPDF(dados)
	if err != nil {
		return nil, fmt.Errorf("erro ao extrair texto: %v", err)
	}

	if len(texto) < 500 {
		return nil, nil
	}

	// Extrair artigos com regex
	artigos, err := regexExtractor.ExtrairArtigos(texto, "Tribunal Supremo")
	if err != nil {
		return nil, fmt.Errorf("erro no regex: %v", err)
	}

	return artigos, nil
}

// ============================================
// CLIENTE HTTP SIMPLES
// ============================================

type httpClient struct {
	timeout time.Duration
}

func (c *httpClient) baixar(url string) ([]byte, error) {
	client := &http.Client{Timeout: c.timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ============================================
// COLETOR DE LINKS (SEQUENCIAL)
// ============================================

func coletarLinks(ctx context.Context, urlsChan chan<- string) {
	fontes := []struct {
		URL  string
		Nome string
	}{
		{"https://www.ts.gov.mz/legislacao/", "Tribunal Supremo"},
		{"https://cfjj.gov.mz/centro-de-documentacao-e-informacao/biblioteca-digital/", "CFJJ"},
		{"https://www.ta.gov.mz/", "Autoridade Tributária"},
	}

	linksEnviados := make(map[string]bool)
	var mu sync.Mutex

	for _, fonte := range fontes {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("📡 Coletando links de: %s", fonte.Nome)

		c := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
			colly.AllowURLRevisit(),
			colly.MaxDepth(2),
		)

		c.SetRequestTimeout(30 * time.Second)
		c.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			RandomDelay: 200 * time.Millisecond,
			Parallelism: 1,
		})

		c.OnHTML("a[href]", func(e *colly.HTMLElement) {
			link := e.Attr("href")
			texto := strings.ToLower(e.Text)

			if !strings.Contains(link, ".pdf") {
				return
			}

			isLei := strings.Contains(texto, "lei") ||
				strings.Contains(texto, "decreto") ||
				strings.Contains(texto, "regulamento") ||
				strings.Contains(texto, "resolução")

			if !isLei {
				return
			}

			// URL absoluta
			if !strings.HasPrefix(link, "http") {
				link = e.Request.AbsoluteURL(link)
			}

			mu.Lock()
			if linksEnviados[link] {
				mu.Unlock()
				return
			}
			linksEnviados[link] = true
			mu.Unlock()

			log.Printf("🔍 %s: Encontrou: %s", fonte.Nome, link)

			select {
			case urlsChan <- link:
			case <-ctx.Done():
				return
			}
		})

		c.OnError(func(r *colly.Response, err error) {
			// Silenciar erros de rede
		})

		c.Visit(fonte.URL)
		c.Wait()
	}

	log.Printf("📊 Total de links únicos encontrados: %d", len(linksEnviados))
}

// ============================================
// EXTRAIR TEXTO DE PDF
// ============================================

func extrairTextoPDF(dados []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return "", fmt.Errorf("erro ao abrir PDF: %v", err)
	}

	var texto strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		content, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		texto.WriteString(content)
		texto.WriteString("\n")
	}

	return texto.String(), nil
}

// ============================================
// FALLBACK LOCAL
// ============================================

func getLeisMocambique() []domain.Artigo {
	now := time.Now()
	return []domain.Artigo{
		{
			ID: "const_art_1", Lei: "Constituição da República de Moçambique",
			LeiNumero: "Constituição 2004", Artigo: "Art. 1",
			Texto:         "A República de Moçambique é um Estado de Direito democrático.",
			PalavrasChave: []string{"constituição", "estado", "democracia", "moçambique"},
			Versao: 1, DataVigencia: time.Date(2004, 12, 21, 0, 0, 0, 0, time.UTC),
			DataPublicacao: time.Date(2004, 12, 21, 0, 0, 0, 0, time.UTC),
			Status: "Vigente", Fonte: "Constituição de Moçambique",
			CriadoEm: now, AtualizadoEm: now, AprovadoPor: "Regex",
		},
	}
}

// ============================================
// SALVAR ARTIGOS (fallback)
// ============================================

func SalvarArtigos(artigos []domain.Artigo, caminho string) error {
	var existentes []domain.Artigo
	if data, err := os.ReadFile(caminho); err == nil {
		json.Unmarshal(data, &existentes)
	}

	idsExistentes := make(map[string]bool)
	for _, a := range existentes {
		idsExistentes[a.ID] = true
	}

	adicionados := 0
	for _, a := range artigos {
		if !idsExistentes[a.ID] {
			existentes = append(existentes, a)
			idsExistentes[a.ID] = true
			adicionados++
		}
	}

	data, err := json.MarshalIndent(existentes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(caminho, data, 0644)
}
