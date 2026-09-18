package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/handler"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	jsonrepo "github.com/wesleysemende133/buscador-juridico/internal/repository/json"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

// ============================================
// MIDDLEWARE CORS
// ============================================

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

// ============================================
// MAIN
// ============================================

func main() {
	caminhoDados := "data/leis.json"
	ctx := context.Background()

	// ===== INICIALIZAR GOOGLE GEMINI =====
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey != "" {
		if err := crawler.InitGeminiExtractor(ctx, apiKey); err != nil {
			log.Printf("⚠️ Erro ao inicializar Gemini: %v", err)
		} else {
			log.Println("🧠 Gemini Extractor inicializado com sucesso!")
		}
	} else {
		log.Println("⚠️  GEMINI_API_KEY não configurada. IA desativada.")
	}

	// ===== INJEÇÃO DE DEPENDÊNCIAS =====
	repo := jsonrepo.NewJSONRepository(caminhoDados)
	idGen := infrastructure.TimestampIDGenerator{}

	adminService := service.NewAdminService(repo, repo, repo, idGen)
	buscaService := service.NewBuscaService(repo)
	limpezaService := service.NewLimpezaService(repo, repo)

	adminHandler := handler.NewAdminHandler(adminService)
	buscaHandler := handler.NewBuscaHandler(buscaService)
	crawlerHandler := handler.NewCrawlerHandler(adminService)
	limpezaHandler := handler.NewLimpezaHandler(limpezaService)

	// ===== ROTAS PÚBLICAS =====
	http.HandleFunc("/buscar", corsMiddleware(buscaHandler.BuscarHandler))
	http.HandleFunc("/artigo/", corsMiddleware(buscaHandler.ArtigoHandler))
	http.HandleFunc("/leis", corsMiddleware(buscaHandler.LeisHandler))

	// ===== ROTAS ADMIN =====
	http.HandleFunc("/admin/artigo", corsMiddleware(adminHandler.AdicionarArtigoHandler))
	http.HandleFunc("/admin/artigo/", corsMiddleware(adminHandler.EditarArtigoHandler))
	http.HandleFunc("/admin/artigos", corsMiddleware(adminHandler.ListarArtigosHandler))
	http.HandleFunc("/admin/artigo/delete/", corsMiddleware(adminHandler.ExcluirArtigoHandler))
	http.HandleFunc("/admin/dedup/stats", corsMiddleware(adminHandler.EstatisticasDedupHandler))

	// ===== ROTAS DE LIMPEZA =====
	http.HandleFunc("/admin/duplicados", corsMiddleware(limpezaHandler.VerificarDuplicadosHandler))
	http.HandleFunc("/admin/limpar", corsMiddleware(limpezaHandler.LimparDuplicadosHandler))

	// ===== ROTA CRAWLER =====
	http.HandleFunc("/crawler/buscar", corsMiddleware(crawlerHandler.BuscarLeisHandler))

	// ===== ROTA RAIZ =====
	http.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "Inativo"
		if os.Getenv("GEMINI_API_KEY") != "" {
			status = "Ativo (Google Gemini)"
		}
		w.Write([]byte(`{
			"mensagem": "Buscador Jurídico API - Moçambique",
			"ia_status": "` + status + `",
			"endpoints": {
				"buscar": "GET /buscar?q=termo",
				"artigo": "GET /artigo/{id}",
				"leis": "GET /leis",
				"admin_listar": "GET /admin/artigos",
				"admin_adicionar": "POST /admin/artigo",
				"admin_editar": "PUT /admin/artigo/{id}",
				"admin_excluir": "DELETE /admin/artigo/delete/{id}",
				"admin_duplicados": "GET /admin/duplicados",
				"admin_limpar": "POST /admin/limpar",
				"crawler": "GET /crawler/buscar"
			}
		}`))
	}))

	log.Println("🚀 Servidor rodando em http://localhost:8080")
	log.Println("📚 Exemplo: http://localhost:8080/buscar?q=guarda")
	log.Println("⚙️  Admin: http://localhost:8080/admin/artigos")
	log.Println("🧹 Duplicados: http://localhost:8080/admin/duplicados")
	log.Println("🕷️  Crawler: http://localhost:8080/crawler/buscar")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
