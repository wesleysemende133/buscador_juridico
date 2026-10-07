package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type AgenteHandler struct {
	agenteService *service.AgenteService
	pool          *service.WorkerPool
}

func NewAgenteHandler(s *service.AgenteService) *AgenteHandler {
	return &AgenteHandler{agenteService: s}
}

// SetPool associa o Worker Pool ao handler
func (h *AgenteHandler) SetPool(pool *service.WorkerPool) {
	h.pool = pool
}

// POST /agente
// Requer autenticação JWT. O email é extraído do token, não do body.
func (h *AgenteHandler) ProcessarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// ============================================
	// EXTRAIR EMAIL DO JWT
	// ============================================
	email, ok := r.Context().Value(auth.EmailKey).(string)
	if !ok || email == "" {
		respErro(w, http.StatusUnauthorized, "sessão inválida. Faz login novamente.")
		return
	}

	// ============================================
	// LER BODY
	// ============================================
	var body struct {
		Pergunta string `json:"pergunta"`
		Modo     string `json:"modo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if body.Pergunta == "" {
		respErro(w, http.StatusBadRequest, "pergunta obrigatória")
		return
	}

	// ============================================
	// USAR WORKER POOL (se disponível)
	// ============================================
	if h.pool != nil {
		resultado, err := h.pool.Submeter(r.Context(), body.Pergunta, body.Modo, email)
		if err != nil {
			respErro(w, http.StatusServiceUnavailable, err.Error())
			return
		}

		if resultado.Erro != nil {
			respErro(w, http.StatusInternalServerError, "erro ao processar pedido")
			return
		}

		respJSON(w, http.StatusOK, map[string]interface{}{
			"resposta":             resultado.Resposta,
			"artigos":              resultado.Artigos,
			"modo":                 body.Modo,
			"perguntas_restantes":  resultado.PerguntasRestantes,
		})
		return
	}

	// ============================================
	// FALLBACK: sem worker pool
	// ============================================
	req := service.AgenteRequest{
		Pergunta: body.Pergunta,
		Modo:     body.Modo,
		Email:    email,
	}

	resposta, err := h.agenteService.Processar(r.Context(), req)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro interno ao processar pedido")
		return
	}

	respJSON(w, http.StatusOK, resposta)
}

// GET /agente/metricas
// Requer JWT + admin
func (h *AgenteHandler) MetricasHandler(w http.ResponseWriter, r *http.Request) {
	if h.pool == nil {
		respErro(w, http.StatusServiceUnavailable, "worker pool não configurado")
		return
	}

	metricas := h.pool.Metricas()

	respJSON(w, http.StatusOK, map[string]interface{}{
		"total_processados": metricas.TotalProcessados,
		"total_erros":       metricas.TotalErros,
		"em_fila":           metricas.EmFila,
		"processando":       metricas.Processando,
	})
}

// POST /agente/stream
// Versão com Server-Sent Events
func (h *AgenteHandler) ProcessarStreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// Verificar se o cliente aceita SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		respErro(w, http.StatusInternalServerError, "streaming não suportado")
		return
	}

	// Headers SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx

	// Extrair email do JWT
	email, ok := r.Context().Value(auth.EmailKey).(string)
	if !ok || email == "" {
		enviarSSE(w, flusher, service.EventoStream{
			Tipo: "erro",
			Dados: map[string]string{"mensagem": "sessão inválida"},
		})
		return
	}

	// Ler body
	var body struct {
		Pergunta string `json:"pergunta"`
		Modo     string `json:"modo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		enviarSSE(w, flusher, service.EventoStream{
			Tipo: "erro",
			Dados: map[string]string{"mensagem": "JSON inválido"},
		})
		return
	}

	// Callback que envia eventos SSE
	callback := func(evento service.EventoStream) {
		enviarSSE(w, flusher, evento)
	}

	// Processar
	req := service.AgenteRequest{
		Pergunta: body.Pergunta,
		Modo:     body.Modo,
		Email:    email,
	}

	if err := h.agenteService.ProcessarComStream(r.Context(), req, callback); err != nil {
		enviarSSE(w, flusher, service.EventoStream{
			Tipo: "erro",
			Dados: map[string]string{"mensagem": err.Error()},
		})
	}
}

// enviarSSE envia um evento no formato SSE
func enviarSSE(w http.ResponseWriter, flusher http.Flusher, evento service.EventoStream) {
	dados, err := json.Marshal(evento)
	if err != nil {
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", dados)
	flusher.Flush()
}
