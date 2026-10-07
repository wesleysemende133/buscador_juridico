package auth

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Cache do secret (lido uma vez no arranque)
var secretCache []byte
var secretCarregado bool

// getSecret devolve o JWT_SECRET obrigatoriamente.
// Se não existir ou for demasiado curto, o servidor NÃO arranca.
func getSecret() []byte {
	if secretCarregado {
		return secretCache
	}

	secret := os.Getenv("JWT_SECRET")

	if secret == "" {
		log.Fatal("❌ JWT_SECRET não está definido. O servidor não pode arrancar sem um secret seguro.")
	}

	if len(secret) < 32 {
		log.Fatalf("❌ JWT_SECRET demasiado curto (%d caracteres). Mínimo: 32.", len(secret))
	}

	secretCache = []byte(secret)
	secretCarregado = true
	return secretCache
}

// ValidarSecret verifica se o JWT_SECRET está configurado.
// Deve ser chamado no arranque do servidor.
func ValidarSecret() error {
	secret := os.Getenv("JWT_SECRET")

	if secret == "" {
		return errors.New("JWT_SECRET não está definido no .env")
	}

	if len(secret) < 32 {
		return errors.New("JWT_SECRET demasiado curto (mínimo 32 caracteres)")
	}

	return nil
}

type Claims struct {
	UserID int    `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GerarToken(userID int, email, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "base-legal",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getSecret())
}

func ValidarToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de assinatura inválido")
		}
		return getSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("token inválido")
}
