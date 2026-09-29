package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type SolicitacoesHandler struct {
	DB *sql.DB
}

func NewSolicitacoesHandler(db *sql.DB) *SolicitacoesHandler {
	return &SolicitacoesHandler{DB: db}
}

type SolicitacaoRequest struct {
	Nome       string `json:"nome"`
	Email      string `json:"email"`
	Telefone   string `json:"telefone"`
	AreaDireito string `json:"area_direito"`
	Descricao  string `json:"descricao"`
	Urgencia   string `json:"urgencia"` // "baixa", "media", "alta"
}

// POST /solicitacoes
func (h *SolicitacoesHandler) CriarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req SolicitacaoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if req.Nome == "" || req.Email == "" || req.Descricao == "" {
		respErro(w, http.StatusBadRequest, "nome, email e descrição obrigatórios")
		return
	}

	var id int
	err := h.DB.QueryRow(
		`INSERT INTO solicitacoes_advogado 
		 (nome, email, telefone, area_direito, descricao, urgencia, status, criado_em)
		 VALUES ($1, $2, $3, $4, $5, $6, 'pendente', NOW())
		 RETURNING id`,
		req.Nome, req.Email, req.Telefone, req.AreaDireito, req.Descricao, req.Urgencia,
	).Scan(&id)

	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao criar solicitação")
		return
	}

	respJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       id,
		"mensagem": "Solicitação enviada com sucesso",
	})
}

// GET /solicitacoes?email=...
func (h *SolicitacoesHandler) ListarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		respErro(w, http.StatusBadRequest, "email obrigatório")
		return
	}

	rows, err := h.DB.Query(
		`SELECT id, nome, email, telefone, area_direito, descricao, urgencia, status, criado_em
		 FROM solicitacoes_advogado
		 WHERE email = $1
		 ORDER BY criado_em DESC`,
		email,
	)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	defer rows.Close()

	type Solicitacao struct {
		ID          int       `json:"id"`
		Nome        string    `json:"nome"`
		Email       string    `json:"email"`
		Telefone    string    `json:"telefone"`
		AreaDireito string    `json:"area_direito"`
		Descricao   string    `json:"descricao"`
		Urgencia    string    `json:"urgencia"`
		Status      string    `json:"status"`
		CriadoEm    time.Time `json:"criado_em"`
	}

	var solicitacoes []Solicitacao
	for rows.Next() {
		var s Solicitacao
		if err := rows.Scan(&s.ID, &s.Nome, &s.Email, &s.Telefone, &s.AreaDireito, &s.Descricao, &s.Urgencia, &s.Status, &s.CriadoEm); err != nil {
			continue
		}
		solicitacoes = append(solicitacoes, s)
	}

	if solicitacoes == nil {
		solicitacoes = []Solicitacao{}
	}

	respJSON(w, http.StatusOK, map[string]interface{}{
		"solicitacoes": solicitacoes,
		"total":        len(solicitacoes),
	})
}
