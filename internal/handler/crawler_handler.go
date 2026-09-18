package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type CrawlerHandler struct {
	adminService *service.AdminService
}

func NewCrawlerHandler(adminService *service.AdminService) *CrawlerHandler {
	return &CrawlerHandler{adminService: adminService}
}

func (h *CrawlerHandler) BuscarLeisHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("🔄 Iniciando busca automática de leis (WORKERS)...")

	ctx := context.Background()

	persistir := func(novos []domain.Artigo) (int, error) {
		return h.adminService.AdicionarVarios(novos)
	}

	_, err := crawler.BuscarLeisPlanalto(ctx, persistir)
	if err != nil {
		log.Printf("❌ Erro: %v", err)
		http.Error(w, "Erro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	total, _ := h.adminService.ContarArtigos()

	log.Printf("✅ Busca concluída | Total no banco: %d", total)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem":       "Busca concluída!",
		"total_no_banco": total,
		"workers":        crawler.NUM_WORKERS,
	})
}
