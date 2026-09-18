package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) AdicionarArtigoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	var artigo domain.Artigo
	if err := json.NewDecoder(r.Body).Decode(&artigo); err != nil {
		http.Error(w, "Erro ao decodificar: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.adminService.AdicionarArtigo(artigo); err != nil {
		http.Error(w, "Erro ao adicionar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Artigo adicionado com sucesso!"})
}

func (h *AdminHandler) ListarArtigosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido. Use GET.", http.StatusMethodNotAllowed)
		return
	}

	artigos, err := h.adminService.ListarArtigos()
	if err != nil {
		http.Error(w, "Erro ao listar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(artigos)
}

func (h *AdminHandler) EditarArtigoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Método não permitido. Use PUT.", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/admin/artigo/"):]
	if id == "" {
		http.Error(w, "ID é obrigatório", http.StatusBadRequest)
		return
	}

	var artigo domain.Artigo
	if err := json.NewDecoder(r.Body).Decode(&artigo); err != nil {
		http.Error(w, "Erro ao decodificar: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.adminService.EditarArtigo(id, artigo); err != nil {
		http.Error(w, "Erro ao editar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Artigo editado com sucesso!"})
}

func (h *AdminHandler) ExcluirArtigoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Método não permitido. Use DELETE.", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/admin/artigo/delete/"):]
	if id == "" {
		http.Error(w, "ID é obrigatório", http.StatusBadRequest)
		return
	}

	if err := h.adminService.ExcluirArtigo(id); err != nil {
		http.Error(w, "Erro ao excluir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Artigo excluído com sucesso!"})
}

// EstatisticasDedupHandler - GET /admin/dedup/stats
func (h *AdminHandler) EstatisticasDedupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	idsCount, hashesCount := h.adminService.EstatisticasDedup()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ids_indexados":    idsCount,
		"hashes_indexados": hashesCount,
		"descricao":        "Índice de deduplicação ativo",
	})
}