package main

import (
	"log"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/handler"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	jsonrepo "github.com/wesleysemende133/buscador-juridico/internal/repository/json"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

func main() {
	caminhoDados := "data/leis.json"

	// ===== INJEÇÃO DE DEPENDÊNCIAS (DIP) =====
	repo := jsonrepo.NewJSONRepository(caminhoDados)
	idGen := infrastructure.TimestampIDGenerator{}

	adminService := service.NewAdminService(repo, repo, repo, idGen)
	buscaService := service.NewBuscaService(repo)

	adminHandler := handler.NewAdminHandler(adminService)
	buscaHandler := handler.NewBuscaHandler(buscaService)

	// ===== CRIAR O CRAWLER HANDLER =====
	// O crawler usa o adminService para adicionar artigos
	crawlerHandler := handler.NewCrawlerHandler(adminService)

	// ===== ROTAS PÚBLICAS =====
	http.HandleFunc("/buscar", buscaHandler.BuscarHandler)
	http.HandleFunc("/artigo/", buscaHandler.ArtigoHandler)
	http.HandleFunc("/leis", buscaHandler.LeisHandler)

	// ===== ROTAS ADMIN =====
	http.HandleFunc("/admin/artigo", adminHandler.AdicionarArtigoHandler)           // POST
	http.HandleFunc("/admin/artigo/", adminHandler.EditarArtigoHandler)             // PUT /admin/artigo/{id}
	http.HandleFunc("/admin/artigos", adminHandler.ListarArtigosHandler)            // GET
	http.HandleFunc("/admin/artigo/delete/", adminHandler.ExcluirArtigoHandler)     // DELETE /admin/artigo/delete/{id}

	// ===== ROTA CRAWLER =====
	http.HandleFunc("/crawler/buscar", crawlerHandler.BuscarLeisHandler)

	// ===== ROTA RAIZ =====
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"mensagem": "Buscador Jurídico API - Moçambique",
			"endpoints": {
				"buscar": "GET /buscar?q=termo",
				"artigo": "GET /artigo/{id}",
				"leis": "GET /leis",
				"admin_listar": "GET /admin/artigos",
				"admin_adicionar": "POST /admin/artigo",
				"admin_editar": "PUT /admin/artigo/{id}",
				"admin_excluir": "DELETE /admin/artigo/delete/{id}",
				"crawler": "GET /crawler/buscar"
			}
		}`))
	})

	log.Println("🚀 Servidor rodando em http://localhost:8080")
	log.Println("📚 Exemplo: http://localhost:8080/buscar?q=guarda")
	log.Println("⚙️  Admin: http://localhost:8080/admin/artigos")
	log.Println("🕷️  Crawler: http://localhost:8080/crawler/buscar")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
