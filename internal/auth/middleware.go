package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// ContextKey para guardar dados do utilizador
type contextKey string

const (
	UserIDKey contextKey = "user_id"
	EmailKey  contextKey = "email"
	RoleKey   contextKey = "role"
)

// MiddlewareJWT protege uma rota exigindo token válido
func MiddlewareJWT(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			responderErro(w, http.StatusUnauthorized, "token ausente")
			return
		}

		partes := strings.SplitN(header, " ", 2)
		if len(partes) != 2 || partes[0] != "Bearer" {
			responderErro(w, http.StatusUnauthorized, "formato inválido. Use: Bearer <token>")
			return
		}

		claims, err := ValidarToken(partes[1])
		if err != nil {
			responderErro(w, http.StatusUnauthorized, "token inválido ou expirado")
			return
		}

		// Guardar no contexto da request
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		ctx = context.WithValue(ctx, EmailKey, claims.Email)
		ctx = context.WithValue(ctx, RoleKey, claims.Role)

		next(w, r.WithContext(ctx))
	}
}

// MiddlewareAdmin exige role=admin
func MiddlewareAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(RoleKey).(string)
		if role != "admin" {
			responderErro(w, http.StatusForbidden, "acesso restrito a administradores")
			return
		}
		next(w, r)
	}
}

func responderErro(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"erro": msg})
}