package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type BuscaHandler struct {
	buscaService *service.BuscaService
}

func NewBuscaHandler(buscaService *service.BuscaService) *BuscaHandler {
	return &BuscaHandler{buscaService: buscaService}
}

func (h *BuscaHandler) BuscarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Parâmetro 'q' é obrigatório", http.StatusBadRequest)
		return
	}

	limite := 20
	if l := r.URL.Query().Get("limite"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limite = v
		}
	}

	categoria := r.URL.Query().Get("categoria")

	resultados, err := h.buscaService.BuscarInteligente(query, categoria, limite)
	if err != nil {
		http.Error(w, "Erro ao buscar: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// CORREÇÃO: Usar o tipo correto
	if resultados == nil {
		resultados = []domain.Artigo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resultados)
}

func (h *BuscaHandler) ArtigoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	json.NewEncoder(w).Encode(artigo)
}

func (h *BuscaHandler) LeisHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
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
	json.NewEncoder(w).Encode(map[string]interface{}{"leis": leis})
}
