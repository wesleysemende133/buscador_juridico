package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type BuscaHandler struct {
	buscaService *service.BuscaService
}

func NewBuscaHandler(buscaService *service.BuscaService) *BuscaHandler {
	return &BuscaHandler{buscaService: buscaService}
}

func (h *BuscaHandler) BuscarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Parâmetro 'q' é obrigatório", http.StatusBadRequest)
		return
	}

	limite := 10
	if l := r.URL.Query().Get("limite"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limite = v
		}
	}

	resultados, err := h.buscaService.Buscar(query, limite)
	if err != nil {
		http.Error(w, "Erro ao buscar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(resultados)
}

func (h *BuscaHandler) ArtigoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/artigo/"):]
	if id == "" {
		http.Error(w, "ID é obrigatório", http.StatusBadRequest)
		return
	}

	artigo, err := h.buscaService.BuscarPorID(id)
	if err != nil {
		http.Error(w, "Erro ao buscar: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if artigo == nil {
		http.Error(w, "Artigo não encontrado", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(artigo)
}

func (h *BuscaHandler) LeisHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	leis, err := h.buscaService.ListarLeis()
	if err != nil {
		http.Error(w, "Erro ao listar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{"leis": leis})
}
