package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type CategoriasHandler struct {
	buscaService *service.BuscaService
}

func NewCategoriasHandler(buscaService *service.BuscaService) *CategoriasHandler {
	return &CategoriasHandler{buscaService: buscaService}
}

// CategoriasHandler - GET /categorias
func (h *CategoriasHandler) ListarCategoriasHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	artigos, err := h.buscaService.ListarTodos()
	if err != nil {
		http.Error(w, "Erro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	contagem := make(map[string]int)
	subcategorias := make(map[string]map[string]int)

	for _, a := range artigos {
		if a.Categoria == "" {
			continue
		}
		contagem[a.Categoria]++
		if subcategorias[a.Categoria] == nil {
			subcategorias[a.Categoria] = make(map[string]int)
		}
		if a.Subcategoria != "" {
			subcategorias[a.Categoria][a.Subcategoria]++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_artigos":  len(artigos),
		"categorias":     contagem,
		"subcategorias":  subcategorias,
	})
}
