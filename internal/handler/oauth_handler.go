package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"google.golang.org/api/idtoken"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
)

type OAuthHandler struct {
	DB             *sql.DB
	GoogleClientID string
}

func NewOAuthHandler(db *sql.DB) *OAuthHandler {
	return &OAuthHandler{
		DB:             db,
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"),
	}
}

type GoogleLoginInput struct {
	IDToken string `json:"id_token"`
}

func (h *OAuthHandler) GoogleLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var input GoogleLoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if input.IDToken == "" {
		respErro(w, http.StatusBadRequest, "id_token obrigatório")
		return
	}

	if h.GoogleClientID == "" {
		respErro(w, http.StatusServiceUnavailable, "Google OAuth não configurado")
		return
	}

	payload, err := idtoken.Validate(r.Context(), input.IDToken, h.GoogleClientID)
	if err != nil {
		respErro(w, http.StatusUnauthorized, "token Google inválido")
		return
	}

	email, _ := payload.Claims["email"].(string)
	nome, _ := payload.Claims["name"].(string)
	sub := payload.Subject

	if email == "" {
		respErro(w, http.StatusBadRequest, "email não fornecido")
		return
	}

	var id int
	var nomeDB, role string
	err = h.DB.QueryRow(
		`SELECT id, nome, role FROM utilizadores WHERE email = $1`,
		email,
	).Scan(&id, &nomeDB, &role)

	if err == sql.ErrNoRows {
		if nome == "" {
			nome = strings.Split(email, "@")[0]
		}

		err = h.DB.QueryRow(
			`INSERT INTO utilizadores (nome, email, senha_hash, role, oauth_provider, oauth_sub)
			 VALUES ($1, $2, $3, 'user', 'google', $4)
			 RETURNING id`,
			nome, email, "oauth-no-password", sub,
		).Scan(&id)
		if err != nil {
			respErro(w, http.StatusInternalServerError, "erro ao criar utilizador")
			return
		}
		nomeDB = nome
		role = "user"
	} else if err != nil {
		respErro(w, http.StatusInternalServerError, "erro na base de dados")
		return
	} else {
		h.DB.Exec(
			`UPDATE utilizadores SET oauth_provider = 'google', oauth_sub = $1 
			 WHERE id = $2 AND (oauth_sub IS NULL OR oauth_sub = '')`,
			sub, id,
		)
	}

	token, err := auth.GerarToken(id, email, role)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao gerar token")
		return
	}

	respJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    id,
			"nome":  nomeDB,
			"email": email,
			"role":  role,
		},
	})
}
