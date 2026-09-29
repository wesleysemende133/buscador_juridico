package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type AgenteHandler struct {
	agenteService *service.AgenteService
}

func NewAgenteHandler(s *service.AgenteService) *AgenteHandler {
	return &AgenteHandler{agenteService: s}
}

// POST /agente
func (h *AgenteHandler) ProcessarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req service.AgenteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	resposta, err := h.agenteService.Processar(r.Context(), req)
	if err != nil {
		respErro(w, http.StatusInternalServerError, err.Error())
		return
	}

	respJSON(w, http.StatusOK, resposta)
}
