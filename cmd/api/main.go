package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/handler"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	"github.com/wesleysemende133/buscador-juridico/internal/middleware"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
	jsonrepo "github.com/wesleysemende133/buscador-juridico/internal/repository/json"
	"github.com/wesleysemende133/buscador-juridico/internal/repository/postgres"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Origins permitidos (configuráveis por env)
		origensPermitidas := getOrigensPermitidas()

		// Verificar se o origin está autorizado
		permitido := false
		for _, o := range origensPermitidas {
			if o == origin {
				permitido = true
				break
			}
		}

		if permitido {
			// Reflectir o origin (não usar *)
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		} else if len(origensPermitidas) == 0 {
			// Fallback: se não há allowlist, usar * (só para dev)
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// ============================================
		// LIMITE DE BODY (1 MB)
		// Protege contra JSON gigante / DoS
		// ============================================
		if r.Body != nil && r.Method != http.MethodGet {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
		}

		next(w, r)
	}
}

// getOrigensPermitidas devolve a lista de origins autorizados
func getOrigensPermitidas() []string {
	env := os.Getenv("CORS_ALLOWED_ORIGINS")
	if env == "" {
		// Fallback para desenvolvimento
		return []string{
			"http://localhost:5173",
			"http://localhost:5174",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:5174",
		}
	}

	var origens []string
	for _, o := range strings.Split(env, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origens = append(origens, o)
		}
	}
	return origens
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env não encontrado")
	}

	// Validar JWT_SECRET antes de continuar
	if err := auth.ValidarSecret(); err != nil {
		log.Fatalf("❌ Erro de configuração: %v", err)
	}
	log.Println("🔐 JWT_SECRET validado com sucesso")

	ctx := context.Background()

	// ============================================
	// RATE LIMITERS (Redis)
	// ============================================
	redisClient, err := middleware.NovoClienteRedis()
	if err != nil {
		log.Fatalf("❌ Erro ao ligar ao Redis: %v", err)
	}
	defer redisClient.Close()

	limiterGlobal := middleware.NovoRateLimiter(redisClient, "global", 120, 30)
	limiterLogin := middleware.NovoRateLimiter(redisClient, "login", 5, 3)
	limiterBusca := middleware.NovoRateLimiter(redisClient, "busca", 60, 15)

	log.Println("🛡️  Rate limiting (Redis) ativo:")
	log.Println("   • Global:  120 req/min")
	log.Println("   • Login:     5 req/min")
	log.Println("   • Busca:    60 req/min")

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
	var pgRepo *postgres.PostgresRepository

	dbURL := os.Getenv("DATABASE_URL")
	useJSON := os.Getenv("USE_JSON") == "true"

	if dbURL != "" && !useJSON {
		var err error
		pgRepo, err = postgres.NewPostgresRepository(dbURL)
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
	// AUTH + OAUTH + VALIDADOR + LIMITES
	// ============================================
	var authHandler *handler.AuthHandler
	var oauthHandler *handler.OAuthHandler
	var validador *service.AuthValidator
	var limitesService *service.LimitesService

	if pgRepo != nil {
		db := pgRepo.DB()
		authHandler = handler.NewAuthHandler(db)
		oauthHandler = handler.NewOAuthHandler(db)
		validador = service.NewAuthValidator(db)
		limitesService = service.NewLimitesService(db)

		log.Println("🔐 Auth JWT ativa")
		log.Println("📊 Serviço de limites + validador ativos")

		if os.Getenv("GOOGLE_CLIENT_ID") != "" {
			log.Println("🔑 Google OAuth ativo")
		} else {
			log.Println("⚠️  Google OAuth desativado (GOOGLE_CLIENT_ID vazio)")
		}
	} else {
		log.Println("⚠️  Auth JWT desativada (só funciona com PostgreSQL)")
	}

	// ===== SERVIÇOS =====
	idGen := infrastructure.TimestampIDGenerator{}
	adminService := service.NewAdminService(repo, buscador, editor, idGen)
	buscaService := service.NewBuscaService(buscador)
	limpezaService := service.NewLimpezaService(repo, buscador)
	agenteService := service.NewAgenteService(buscador, limitesService, validador)

	// ============================================
	// WORKER POOL (5 workers, fila de 100)
	// ============================================
	pool := service.NovoWorkerPool(agenteService, 5, 100)
	defer pool.Parar()

	// ===== HANDLERS =====
	adminHandler := handler.NewAdminHandler(adminService)
	buscaHandler := handler.NewBuscaHandler(buscaService)
	crawlerHandler := handler.NewCrawlerHandler(adminService)
	limpezaHandler := handler.NewLimpezaHandler(limpezaService)
	categoriasHandler := handler.NewCategoriasHandler(buscaService)
	agenteHandler := handler.NewAgenteHandler(agenteService)
	agenteHandler.SetPool(pool)

	var planosHandler *handler.PlanosHandler
	if limitesService != nil && validador != nil {
		planosHandler = handler.NewPlanosHandler(limitesService, validador)
	}

	var solicitacoesHandler *handler.SolicitacoesHandler
	if pgRepo != nil {
		solicitacoesHandler = handler.NewSolicitacoesHandler(pgRepo.DB())
	}

	// ============================================
	// ROTAS PÚBLICAS
	// ============================================
	http.HandleFunc("/buscar", corsMiddleware(limiterBusca.Middleware(buscaHandler.BuscarHandler)))
	http.HandleFunc("/artigo/", corsMiddleware(limiterGlobal.Middleware(buscaHandler.ArtigoHandler)))
	http.HandleFunc("/leis", corsMiddleware(limiterGlobal.Middleware(buscaHandler.LeisHandler)))
	http.HandleFunc("/categorias", corsMiddleware(limiterGlobal.Middleware(categoriasHandler.ListarCategoriasHandler)))
	http.HandleFunc("/agente", corsMiddleware(limiterBusca.Middleware(auth.MiddlewareJWT(agenteHandler.ProcessarHandler))))
	http.HandleFunc("/agente/metricas", corsMiddleware(limiterGlobal.Middleware(auth.MiddlewareJWT(auth.MiddlewareAdmin(agenteHandler.MetricasHandler)))))
	http.HandleFunc("/agente/stream", corsMiddleware(limiterBusca.Middleware(auth.MiddlewareJWT(agenteHandler.ProcessarStreamHandler))))

	// ===== AUTH =====
	if authHandler != nil {
		http.HandleFunc("/login", corsMiddleware(limiterLogin.Middleware(authHandler.LoginHandler)))
		http.HandleFunc("/registar", corsMiddleware(limiterLogin.Middleware(authHandler.RegistarHandler)))
		log.Println("🔐 Rotas /login e /registar ativas (5 req/min)")
	}

	// ===== OAUTH GOOGLE =====
	if oauthHandler != nil && os.Getenv("GOOGLE_CLIENT_ID") != "" {
		http.HandleFunc("/auth/google", corsMiddleware(limiterLogin.Middleware(oauthHandler.GoogleLoginHandler)))
		log.Println("🔑 Rota /auth/google ativa")
	}

	// ===== PLANOS =====
	if planosHandler != nil {
		// /planos — PÚBLICO (só lista os planos, sem dados sensíveis)
		http.HandleFunc("/planos", corsMiddleware(limiterGlobal.Middleware(planosHandler.ListarPlanosHandler)))

		// Rotas protegidas com JWT (email vem do token, não do cliente)
		http.HandleFunc("/planos/uso", corsMiddleware(limiterGlobal.Middleware(auth.MiddlewareJWT(planosHandler.UsoHandler))))
		http.HandleFunc("/planos/uso-cidadao", corsMiddleware(limiterGlobal.Middleware(auth.MiddlewareJWT(planosHandler.UsoCidadaoHandler))))
		http.HandleFunc("/planos/actualizar", corsMiddleware(limiterGlobal.Middleware(auth.MiddlewareJWT(planosHandler.ActualizarHandler))))

		log.Println("💳 Rotas /planos ativas (protegidas com JWT)")
	}

	// ===== SOLICITAÇÕES =====
	if solicitacoesHandler != nil {
		http.HandleFunc("/solicitacoes", corsMiddleware(limiterGlobal.Middleware(solicitacoesHandler.CriarHandler)))
		http.HandleFunc("/solicitacoes/listar", corsMiddleware(limiterGlobal.Middleware(solicitacoesHandler.ListarHandler)))
		log.Println("📋 Rotas /solicitacoes ativas")
	}

	// ============================================
	// ROTAS PROTEGIDAS (JWT + admin)
	// ============================================
	protegido := func(h http.HandlerFunc) http.HandlerFunc {
		if authHandler == nil {
			return corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"erro":"autenticação não configurada"}`))
			})
		}
		return corsMiddleware(limiterGlobal.Middleware(auth.MiddlewareJWT(auth.MiddlewareAdmin(h))))
	}

	http.HandleFunc("/admin/artigo", protegido(adminHandler.AdicionarArtigoHandler))
	http.HandleFunc("/admin/artigo/", protegido(adminHandler.EditarArtigoHandler))
	http.HandleFunc("/admin/artigos", protegido(adminHandler.ListarArtigosHandler))
	http.HandleFunc("/admin/artigo/delete/", protegido(adminHandler.ExcluirArtigoHandler))
	http.HandleFunc("/admin/duplicados", protegido(limpezaHandler.VerificarDuplicadosHandler))
	http.HandleFunc("/admin/limpar", protegido(limpezaHandler.LimparDuplicadosHandler))
	http.HandleFunc("/crawler/buscar", protegido(crawlerHandler.BuscarLeisHandler))

	// ============================================
	// HEALTH CHECK
	// ============================================
	http.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))

	// ============================================
	// RAIZ
	// ============================================
	http.HandleFunc("/", corsMiddleware(limiterGlobal.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		db := "JSON"
		if dbURL != "" && !useJSON {
			db = "PostgreSQL"
		}
		authStatus := "desativada"
		if authHandler != nil {
			authStatus = "JWT ativo"
		}
		oauthStatus := "desativado"
		if oauthHandler != nil && os.Getenv("GOOGLE_CLIENT_ID") != "" {
			oauthStatus = "Google ativo"
		}
		w.Write([]byte(`{
			"mensagem": "Base Legal API - Moçambique",
			"versao": "3.0",
			"database": "` + db + `",
			"auth": "` + authStatus + `",
			"oauth": "` + oauthStatus + `",
			"ratelimit": "ativo"
		}`))
	})))

	log.Println("🚀 Servidor rodando em http://localhost:8080")

	// ============================================
	// HTTP SERVER COM TIMEOUTS
	// Protege contra Slowloris e conexões lentas
	// ============================================
	server := &http.Server{
		Addr:              ":8080",
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,   // tempo para ler headers
		ReadTimeout:       15 * time.Second,  // tempo total de leitura
		WriteTimeout:      60 * time.Second,  // tempo de escrita (Gemini pode demorar)
		IdleTimeout:       120 * time.Second, // tempo de conexão idle
	}

	log.Fatal(server.ListenAndServe())
}
