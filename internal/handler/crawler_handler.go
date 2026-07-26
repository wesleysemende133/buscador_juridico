package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type CrawlerHandler struct {
	adminService *service.AdminService
}

func NewCrawlerHandler(adminService *service.AdminService) *CrawlerHandler {
	return &CrawlerHandler{adminService: adminService}
}

// BuscarLeisHandler - GET /crawler/buscar
func (h *CrawlerHandler) BuscarLeisHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	log.Println("🔄 Iniciando busca automática de leis...")

	// Busca as leis usando o crawler
	artigos, err := crawler.BuscarLeisPlanalto()
	if err != nil {
		log.Printf("❌ Erro ao buscar leis: %v", err)
		http.Error(w, "Erro ao buscar leis: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Encontrou %d artigos", len(artigos))

	// Adiciona cada artigo encontrado
	adicionados := 0
	for _, artigo := range artigos {
		if err := h.adminService.AdicionarArtigo(artigo); err != nil {
			log.Printf("⚠️ Erro ao adicionar artigo: %v", err)
		} else {
			adicionados++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem":            "Busca concluída!",
		"artigos_encontrados": len(artigos),
		"artigos_adicionados": adicionados,
	})
}
