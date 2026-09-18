package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type LimpezaHandler struct {
	limpezaService *service.LimpezaService
}

func NewLimpezaHandler(limpezaService *service.LimpezaService) *LimpezaHandler {
	return &LimpezaHandler{limpezaService: limpezaService}
}

// VerificarDuplicadosHandler - GET /admin/duplicados
func (h *LimpezaHandler) VerificarDuplicadosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	resultado, err := h.limpezaService.VerificarDuplicados()
	if err != nil {
		http.Error(w, "Erro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultado)
}

// LimparDuplicadosHandler - POST /admin/limpar
func (h *LimpezaHandler) LimparDuplicadosHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	antes, depois, err := h.limpezaService.LimparDuplicados()
	if err != nil {
		http.Error(w, "Erro: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem":            "Limpeza concluída!",
		"artigos_antes":       antes,
		"artigos_depois":      depois,
		"duplicados_removidos": antes - depois,
	})
}
