package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
)

type AuthHandler struct {
	DB *sql.DB
}

func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{DB: db}
}

type loginInput struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type registoInput struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// POST /registar
func (h *AuthHandler) RegistarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input registoInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if input.Nome == "" || input.Email == "" || len(input.Senha) < 6 {
		respErro(w, http.StatusBadRequest, "nome, email e senha (min 6) obrigatórios")
		return
	}

	hash, err := auth.HashSenha(input.Senha)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}

	var id int
	err = h.DB.QueryRow(
		`INSERT INTO utilizadores (nome, email, senha_hash, role) 
		 VALUES ($1, $2, $3, 'user') RETURNING id`,
		input.Nome, input.Email, hash,
	).Scan(&id)

	if err != nil {
		respErro(w, http.StatusConflict, "email já registado")
		return
	}

	respJSON(w, http.StatusCreated, map[string]interface{}{
		"id":       id,
		"mensagem": "utilizador criado",
	})
}

// POST /login
func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input loginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	var id int
	var nome, hash, role string
	err := h.DB.QueryRow(
		`SELECT id, nome, senha_hash, role FROM utilizadores WHERE email = $1`,
		input.Email,
	).Scan(&id, &nome, &hash, &role)

	if err != nil {
		respErro(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}

	if !auth.VerificarSenha(hash, input.Senha) {
		respErro(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}

	token, err := auth.GerarToken(id, input.Email, role)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao gerar token")
		return
	}

	respJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    id,
			"nome":  nome,
			"email": input.Email,
			"role":  role,
		},
	})
}

// Helpers
func respJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respErro(w http.ResponseWriter, status int, msg string) {
	respJSON(w, status, map[string]string{"erro": msg})
}