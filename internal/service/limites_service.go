package service

import (
	"database/sql"
	"fmt"
	"time"
)

const LimiteGratuitoProfissional = 3

type LimitesService struct {
	DB *sql.DB
}

func NewLimitesService(db *sql.DB) *LimitesService {
	return &LimitesService{DB: db}
}

// Verifica se o utilizador pode fazer uma pergunta no modo profissional
func (s *LimitesService) PodePerguntar(email string, modo string) (bool, int, int, error) {
	if email == "" {
		// Sem email = sem limite (utilizador não autenticado)
		return true, 0, LimiteGratuitoProfissional, nil
	}

	if modo != "profissional" {
		// Modo cidadão é sempre gratuito e ilimitado
		return true, 0, -1, nil
	}

	var contagem int
	var plano string

	err := s.DB.QueryRow(
		`SELECT contagem, plano FROM uso_agente WHERE email = $1 AND modo = $2`,
		email, modo,
	).Scan(&contagem, &plano)

	if err == sql.ErrNoRows {
		// Primeira vez
		_, err := s.DB.Exec(
			`INSERT INTO uso_agente (email, modo, contagem, plano)
			 VALUES ($1, $2, 0, 'gratuito')`,
			email, modo,
		)
		if err != nil {
			return false, 0, LimiteGratuitoProfissional, err
		}
		return true, 0, LimiteGratuitoProfissional, nil
	}

	if err != nil {
		return false, 0, LimiteGratuitoProfissional, err
	}

	// Se for plano pago, sem limite
	if plano != "gratuito" {
		return true, contagem, -1, nil
	}

	// Plano gratuito: limite de 3
	pode := contagem < LimiteGratuitoProfissional
	return pode, contagem, LimiteGratuitoProfissional, nil
}

// Regista o uso (incrementa contador)
func (s *LimitesService) RegistarUso(email string, modo string) error {
	if email == "" || modo != "profissional" {
		return nil
	}

	_, err := s.DB.Exec(
		`UPDATE uso_agente 
		 SET contagem = contagem + 1, ultimo_uso = NOW()
		 WHERE email = $1 AND modo = $2`,
		email, modo,
	)
	return err
}

// Obtém o uso actual
func (s *LimitesService) ObterUso(email string) (map[string]interface{}, error) {
	if email == "" {
		return map[string]interface{}{
			"contagem": 0,
			"limite":   LimiteGratuitoProfissional,
			"plano":    "anonimo",
			"restante": LimiteGratuitoProfissional,
		}, nil
	}

	var contagem int
	var plano string
	var primeiro, ultimo time.Time

	err := s.DB.QueryRow(
		`SELECT contagem, plano, primeira_uso, ultimo_uso 
		 FROM uso_agente WHERE email = $1 AND modo = 'profissional'`,
		email,
	).Scan(&contagem, &plano, &primeiro, &ultimo)

	if err == sql.ErrNoRows {
		return map[string]interface{}{
			"contagem": 0,
			"limite":   LimiteGratuitoProfissional,
			"plano":    "gratuito",
			"restante": LimiteGratuitoProfissional,
		}, nil
	}

	if err != nil {
		return nil, err
	}

	limite := LimiteGratuitoProfissional
	restante := limite - contagem
	if plano != "gratuito" {
		limite = -1 // ilimitado
		restante = -1
	}

	if restante < 0 {
		restante = 0
	}

	return map[string]interface{}{
		"contagem":      contagem,
		"limite":        limite,
		"plano":         plano,
		"restante":      restante,
		"primeiro_uso":  primeiro.Format(time.RFC3339),
		"ultimo_uso":    ultimo.Format(time.RFC3339),
	}, nil
}

// Actualiza o plano do utilizador
func (s *LimitesService) ActualizarPlano(email string, plano string) error {
	_, err := s.DB.Exec(
		`INSERT INTO uso_agente (email, modo, contagem, plano)
		 VALUES ($1, 'profissional', 0, $2)
		 ON CONFLICT (email, modo) 
		 DO UPDATE SET plano = $2, contagem = 0`,
		email, plano,
	)
	return err
}

// Verifica se o plano é válido
func ValidarPlano(plano string) error {
	validos := map[string]bool{
		"gratuito":    true,
		"estudante":   true,
		"profissional": true,
		"empresarial": true,
	}
	if !validos[plano] {
		return fmt.Errorf("plano inválido: %s", plano)
	}
	return nil
}
