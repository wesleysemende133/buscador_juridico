package crawler

import (
	"bytes"
	"context"
	"crypto/tls"
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

const NUM_WORKERS = 5

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
// BUSCAR LEIS
// ============================================

func BuscarLeisPlanalto(ctx context.Context, persistir func([]domain.Artigo) (int, error)) ([]domain.Artigo, error) {
	if regexExtractor == nil {
		InitRegexExtractor()
	}

	muPersistidos.Lock()
	totalPersistidos = 0
	muPersistidos.Unlock()

	log.Printf("🕷️  Iniciando crawler com %d workers paralelos...", NUM_WORKERS)

	urlsChan := make(chan string, 100)
	resultadosChan := make(chan int, 1000)

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

	go func() {
		defer close(urlsChan)
		coletarLinks(ctx, urlsChan)
	}()

	go func() {
		wg.Wait()
		close(resultadosChan)
	}()

	total := 0
	for count := range resultadosChan {
		total += count
	}

	log.Printf("✅ Total de artigos extraídos: %d | Persistidos: %d", total, GetTotalPersistidos())
	return nil, nil
}

// ============================================
// WORKER
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
// PROCESSAR PDF
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

	artigos, err := regexExtractor.ExtrairArtigos(texto, "Fonte Externa")
	if err != nil {
		log.Printf("   ❌ Worker %d: erro no regex: %v", workerID, err)
		return nil, fmt.Errorf("erro no regex: %v", err)
	}

	log.Printf("   ✅ Worker %d: %d artigos extraídos", workerID, len(artigos))
	return artigos, nil
}

// ============================================
// HTTP CLIENT — com TLS flexível
// ============================================

type httpClient struct {
	timeout time.Duration
}

func (c *httpClient) baixar(url string) ([]byte, error) {
	// ⚠️ InsecureSkipVerify: true permite sites com certificados SSL inválidos
	// (como a Imprensa Nacional de Moçambique)
	// Em produção, o ideal é configurar o CA bundle correcto
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Headers realistas (evita bloqueio de bots)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/pdf,*/*")
	req.Header.Set("Accept-Language", "pt-PT,pt;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.google.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d (%s)", resp.StatusCode, resp.Status)
	}

	dados, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Verificar se é PDF (magic bytes %PDF)
	if len(dados) < 5 || !bytes.HasPrefix(dados, []byte("%PDF")) {
		if bytes.Contains(dados, []byte("Cloudflare")) ||
			bytes.Contains(dados, []byte("Human verification")) ||
			bytes.Contains(dados, []byte("cf-browser-verification")) {
			return nil, fmt.Errorf("bloqueado por Cloudflare")
		}
		return nil, fmt.Errorf("não é PDF (%d bytes)", len(dados))
	}

	return dados, nil
}

// ============================================
// COLETOR DE LINKS
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
			RandomDelay: 500 * time.Millisecond, // delay maior para evitar bloqueios
			Parallelism: 1,
		})

		collector.OnRequest(func(r *colly.Request) {
			r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			r.Headers.Set("Accept-Language", "pt-PT,pt;q=0.9,en;q=0.8")
			r.Headers.Set("Referer", "https://www.google.com/")
			r.Headers.Set("DNT", "1")
			r.Headers.Set("Connection", "keep-alive")
			r.Headers.Set("Upgrade-Insecure-Requests", "1")
		})

		collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
			totalLinks++
			link := e.Attr("href")

			// Aceitar PDFs de várias formas
			isPDF := strings.Contains(strings.ToLower(link), ".pdf") ||
				strings.Contains(strings.ToLower(link), "baixar") ||
				strings.Contains(strings.ToLower(link), "download") ||
				strings.Contains(strings.ToLower(link), "/pdf/")

			if !isPDF {
				return
			}

			if !strings.HasPrefix(link, "http") {
				link = e.Request.AbsoluteURL(link)
			}

			// Normalizar
			link = strings.Replace(link, "http://", "https://", 1)

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
			log.Printf("   ⚠️  [%s] Erro: %v", fonte.Nome, err)
		})

		collector.OnScraped(func(r *colly.Response) {
			log.Printf("   📊 [%s] %d links, %d PDFs", fonte.Nome, totalLinks, totalPDFs)
		})

		// Ignorar TLS inválido (Imprensa Nacional)
		collector.SetClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 45 * time.Second,
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

// ============================================
// FUNÇÕES PÚBLICAS (para processamento manual)
// ============================================

func ExtrairTextoPDFPublico(dados []byte) (string, error) {
	return extrairTextoPDF(dados)
}

func ExtrairArtigosPublico(texto string, fonte string) ([]domain.Artigo, error) {
	if regexExtractor == nil {
		InitRegexExtractor()
	}
	return regexExtractor.ExtrairArtigos(texto, fonte)
}
