package service

import (
	"database/sql"
	"fmt"
)

type AuthValidator struct {
	DB *sql.DB
}

func NewAuthValidator(db *sql.DB) *AuthValidator {
	return &AuthValidator{DB: db}
}

// Verifica se o email pertence a um utilizador registado
func (v *AuthValidator) UtilizadorExiste(email string) (bool, error) {
	if email == "" {
		return false, nil
	}

	var id int
	err := v.DB.QueryRow(
		`SELECT id FROM utilizadores WHERE email = $1`,
		email,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Obtém o role do utilizador
func (v *AuthValidator) ObterRole(email string) (string, error) {
	if email == "" {
		return "", fmt.Errorf("email vazio")
	}

	var role string
	err := v.DB.QueryRow(
		`SELECT role FROM utilizadores WHERE email = $1`,
		email,
	).Scan(&role)

	if err == sql.ErrNoRows {
		return "", fmt.Errorf("utilizador não encontrado")
	}
	return role, err
}
