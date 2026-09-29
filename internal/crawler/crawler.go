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
const NUM_WORKERS = 3  

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

	client := &httpClient{timeout: 60 * time.Second}
	dados, err := client.baixar(url)
	if err != nil {
		log.Printf("   ❌ Worker %d: erro ao baixar: %v", workerID, err)
		return nil, fmt.Errorf("erro ao baixar: %v", err)
	}

	log.Printf("   📦 Worker %d: %d bytes recebidos", workerID, len(dados))

	texto, err := extrairTextoPDF(dados)
	if err != nil {
		log.Printf("   ❌ Worker %d: erro ao extrair PDF: %v", workerID, err)
		return nil, fmt.Errorf("erro ao extrair texto: %v", err)
	}

	if len(texto) < 500 {
		log.Printf("   ⚠️  Worker %d: texto muito curto (%d chars)", workerID, len(texto))
		return nil, nil
	}

	log.Printf("   📝 Worker %d: %d chars extraídos", workerID, len(texto))

	artigos, err := regexExtractor.ExtrairArtigos(texto, "Tribunal Supremo")
	if err != nil {
		log.Printf("   ❌ Worker %d: erro no regex: %v", workerID, err)
		return nil, fmt.Errorf("erro no regex: %v", err)
	}

	log.Printf("   ✅ Worker %d: %d artigos extraídos", workerID, len(artigos))
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

	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/pdf,*/*")
	req.Header.Set("Accept-Language", "pt-PT,pt;q=0.9")

	// 🆕 Referer ajuda a passar por alguns bloqueios
	req.Header.Set("Referer", "https://www.ts.gov.mz/legislacao/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 🆕 Verificar status HTTP
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d (%s)", resp.StatusCode, resp.Status)
	}

	dados, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 🆕 Verificar se é realmente PDF (magic bytes "%PDF")
	if len(dados) < 5 || !bytes.HasPrefix(dados, []byte("%PDF")) {
		return nil, fmt.Errorf("não é PDF (tamanho: %d bytes)", len(dados))
	}

	return dados, nil
}

// ============================================
// COLETOR DE LINKS (SEQUENCIAL)
// ============================================

func coletarLinks(ctx context.Context, urlsChan chan<- string) {
	fontes := FontesActivas()
	linksEnviados := make(map[string]bool)
	var mu sync.Mutex

	log.Printf("📡 A coletar de %d fontes activas...", len(fontes))

	for _, fonte := range fontes {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Printf("🌐 [%s] A visitar: %s", fonte.Nome, fonte.URL)

		var totalLinks, totalPDFs int

		collector := colly.NewCollector(
			colly.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
			colly.AllowURLRevisit(),
			colly.MaxDepth(fonte.MaxDepth),
			colly.Async(false),
		)

		collector.SetRequestTimeout(45 * time.Second)
		collector.Limit(&colly.LimitRule{
			DomainGlob:  "*",
			RandomDelay: 200 * time.Millisecond,
			Parallelism: 1,
		})

		collector.OnRequest(func(r *colly.Request) {
			r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			r.Headers.Set("Accept-Language", "pt-PT,pt;q=0.9,en;q=0.8")
		})

		collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			totalLinks++
			link := e.Attr("href")

			// ============================================
			// 1. Só aceitar PDFs
			// ============================================
			if !strings.Contains(strings.ToLower(link), ".pdf") {
				return
			}

			// ============================================
			// 2. URL absoluta
			// ============================================
			if !strings.HasPrefix(link, "http") {
				link = e.Request.AbsoluteURL(link)
			}

			// ============================================
			// 3. LIMPAR caracteres invisíveis (Unicode Private Use)
			// ============================================
			link = strings.Map(func(r rune) rune {
				// \uE000-\uF8FF são caracteres privados (espaços invisíveis do TS)
				if r >= 0xE000 && r <= 0xF8FF {
					return -1
				}
				// \uFFFD é o caracter de substituição (encoding quebrado)
				if r == 0xFFFD {
					return -1
				}
				return r
			}, link)

			// ============================================
			// 4. NORMALIZAR: forçar HTTPS + www
			// ============================================
			link = strings.Replace(link, "http://", "https://", 1)
			link = strings.Replace(link, "https://ts.gov.mz", "https://www.ts.gov.mz", 1)

			// ============================================
			// 5. Evitar duplicados
			// ============================================
			mu.Lock()
			if linksEnviados[link] {
				mu.Unlock()
				return
			}
			linksEnviados[link] = true
			mu.Unlock()

			totalPDFs++
			log.Printf("   ✅ [%s] PDF #%d: %s", fonte.Nome, totalPDFs, link)

			select {
			case urlsChan <- link:
			case <-ctx.Done():
				return
			}
		})

		collector.OnError(func(r *colly.Response, err error) {
			log.Printf("   ⚠️  [%s] Erro em %s: %v", fonte.Nome, r.Request.URL, err)
		})

		collector.OnScraped(func(r *colly.Response) {
			log.Printf("   📊 [%s] %d links, %d PDFs", fonte.Nome, totalLinks, totalPDFs)
		})

		if err := collector.Visit(fonte.URL); err != nil {
			log.Printf("   ❌ [%s] Não conseguiu visitar: %v", fonte.Nome, err)
		}
		collector.Wait()
	}

	log.Printf("📊 Total de PDFs únicos: %d", len(linksEnviados))
}

// ============================================
// EXTRAIR TEXTO DE PDF
// ============================================

func extrairTextoPDF(dados []byte) (texto string, err error) {
	// 🛡️ RECOVER: se a lib pdf entrar em panic, devolvemos erro em vez de crashar
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic ao ler PDF: %v", r)
			texto = ""
		}
	}()

	reader, err := pdf.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return "", fmt.Errorf("erro ao abrir PDF: %v", err)
	}

	var sb strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		// 🛡️ Proteger cada página individualmente
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("   ⚠️  Página %d causou panic (ignorada): %v", i, r)
				}
			}()

			page := reader.Page(i)
			if page.V.IsNull() {
				return
			}
			content, err := page.GetPlainText(nil)
			if err != nil {
				return
			}
			sb.WriteString(content)
			sb.WriteString("\n")
		}()
	}

	return sb.String(), nil
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
