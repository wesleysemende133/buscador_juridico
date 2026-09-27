package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/handler"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
	jsonrepo "github.com/wesleysemende133/buscador-juridico/internal/repository/json"
	"github.com/wesleysemende133/buscador-juridico/internal/repository/postgres"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env não encontrado")
	}

	ctx := context.Background()

	// ===== INICIALIZAR GEMINI =====
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey != "" {
		if err := crawler.InitGeminiExtractor(ctx, apiKey); err != nil {
			log.Printf("⚠️ Erro ao inicializar Gemini: %v", err)
		} else {
			log.Println("🧠 Gemini Extractor inicializado!")
		}
	}

	// ============================================
	// ESCOLHER REPOSITÓRIO
	// ============================================
	var repo repository.Repository
	var buscador repository.Buscador
	var editor repository.Editor

	dbURL := os.Getenv("DATABASE_URL")
	useJSON := os.Getenv("USE_JSON") == "true"

	if dbURL != "" && !useJSON {
		pgRepo, err := postgres.NewPostgresRepository(dbURL)
		if err != nil {
			log.Printf("⚠️  Erro PostgreSQL: %v", err)
			log.Println("📌 Usando JSON como fallback...")
			jRepo := jsonrepo.NewJSONRepository("data/leis.json")
			repo, buscador, editor = jRepo, jRepo, jRepo
		} else {
			log.Println("✅ Usando PostgreSQL")
			repo, buscador, editor = pgRepo, pgRepo, pgRepo
		}
	} else {
		log.Println("📌 Usando JSON")
		jRepo := jsonrepo.NewJSONRepository("data/leis.json")
		repo, buscador, editor = jRepo, jRepo, jRepo
	}

	// ============================================
	// AUTH (reutiliza DB do repositório Postgres)
	// ============================================
	var authHandler *handler.AuthHandler

	if pgRepo, ok := repo.(*postgres.PostgresRepository); ok {
		authHandler = handler.NewAuthHandler(pgRepo.DB())
		log.Println("🔐 Auth JWT ativa")
	} else {
		log.Println("⚠️  Auth JWT desativada (só funciona com PostgreSQL)")
	}

	// ===== SERVIÇOS =====
	idGen := infrastructure.TimestampIDGenerator{}
	adminService := service.NewAdminService(repo, buscador, editor, idGen)
	buscaService := service.NewBuscaService(buscador)
	limpezaService := service.NewLimpezaService(repo, buscador)

	// ===== HANDLERS =====
	adminHandler := handler.NewAdminHandler(adminService)
	buscaHandler := handler.NewBuscaHandler(buscaService)
	crawlerHandler := handler.NewCrawlerHandler(adminService)
	limpezaHandler := handler.NewLimpezaHandler(limpezaService)
	categoriasHandler := handler.NewCategoriasHandler(buscaService)

	// ===== ROTAS PÚBLICAS =====
	http.HandleFunc("/buscar", corsMiddleware(buscaHandler.BuscarHandler))
	http.HandleFunc("/artigo/", corsMiddleware(buscaHandler.ArtigoHandler))
	http.HandleFunc("/leis", corsMiddleware(buscaHandler.LeisHandler))
	http.HandleFunc("/categorias", corsMiddleware(categoriasHandler.ListarCategoriasHandler))

	// ===== AUTH (públicas) =====
	if authHandler != nil {
		http.HandleFunc("/login", corsMiddleware(authHandler.LoginHandler))
		http.HandleFunc("/registar", corsMiddleware(authHandler.RegistarHandler))
		log.Println("🔐 Rotas /login e /registar ativas")
	}

	// ===== ROTAS PROTEGIDAS (JWT + admin) =====
	protegido := func(h http.HandlerFunc) http.HandlerFunc {
		if authHandler == nil {
			return corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"erro":"autenticação não configurada"}`))
			})
		}
		return corsMiddleware(auth.MiddlewareJWT(auth.MiddlewareAdmin(h)))
	}

	http.HandleFunc("/admin/artigo", protegido(adminHandler.AdicionarArtigoHandler))
	http.HandleFunc("/admin/artigo/", protegido(adminHandler.EditarArtigoHandler))
	http.HandleFunc("/admin/artigos", protegido(adminHandler.ListarArtigosHandler))
	http.HandleFunc("/admin/artigo/delete/", protegido(adminHandler.ExcluirArtigoHandler))
	http.HandleFunc("/admin/duplicados", protegido(limpezaHandler.VerificarDuplicadosHandler))
	http.HandleFunc("/admin/limpar", protegido(limpezaHandler.LimparDuplicadosHandler))
	http.HandleFunc("/crawler/buscar", protegido(crawlerHandler.BuscarLeisHandler))

	// ===== RAIZ =====
	http.HandleFunc("/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db := "JSON"
		if dbURL != "" && !useJSON {
			db = "PostgreSQL"
		}
		authStatus := "desativada"
		if authHandler != nil {
			authStatus = "JWT ativo"
		}
		w.Write([]byte(`{
			"mensagem": "Base Legal API - Moçambique",
			"versao": "2.0",
			"database": "` + db + `",
			"auth": "` + authStatus + `"
		}`))
	}))

	log.Println("🚀 Servidor rodando em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
