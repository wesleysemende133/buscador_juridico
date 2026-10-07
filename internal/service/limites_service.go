package service

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	LimiteGratuitoProfissional = 3
	LimiteDiarioCidadao        = 20
)

type LimitesService struct {
	DB *sql.DB
}

func NewLimitesService(db *sql.DB) *LimitesService {
	return &LimitesService{DB: db}
}

// ============================================
// CIDADÃO — 20/dia (renova à meia-noite)
// ============================================

func (s *LimitesService) PodePerguntarCidadao(email string) (bool, int, int, error) {
	if email == "" {
		return false, 0, LimiteDiarioCidadao, nil
	}

	// Procurar registo do dia actual
	var contagem int
	err := s.DB.QueryRow(
		`SELECT contagem FROM uso_agente 
		 WHERE email = $1 AND modo = 'cidadao' AND DATE(ultimo_uso) = CURRENT_DATE`,
		email,
	).Scan(&contagem)

	if err == sql.ErrNoRows {
		// Primeira pergunta hoje
		_, err := s.DB.Exec(
			`INSERT INTO uso_agente (email, modo, contagem, plano, primeira_uso, ultimo_uso)
			 VALUES ($1, 'cidadao', 0, 'gratuito', NOW(), NOW())
			 ON CONFLICT (email, modo) 
			 DO UPDATE SET contagem = 0, ultimo_uso = NOW()`,
			email,
		)
		if err != nil {
			return false, 0, LimiteDiarioCidadao, err
		}
		return true, 0, LimiteDiarioCidadao, nil
	}

	if err != nil {
		return false, 0, LimiteDiarioCidadao, err
	}

	// Se a última utilização foi em dia diferente, resetar
	var ultimoUso time.Time
	s.DB.QueryRow(
		`SELECT ultimo_uso FROM uso_agente WHERE email = $1 AND modo = 'cidadao'`,
		email,
	).Scan(&ultimoUso)

	if ultimoUso.Format("2006-01-02") != time.Now().Format("2006-01-02") {
		// Reset diário
		s.DB.Exec(
			`UPDATE uso_agente SET contagem = 0, ultimo_uso = NOW() 
			 WHERE email = $1 AND modo = 'cidadao'`,
			email,
		)
		contagem = 0
	}

	pode := contagem < LimiteDiarioCidadao
	return pode, contagem, LimiteDiarioCidadao, nil
}

func (s *LimitesService) RegistarUsoCidadao(email string) error {
	if email == "" {
		return nil
	}

	_, err := s.DB.Exec(
		`UPDATE uso_agente 
		 SET contagem = contagem + 1, ultimo_uso = NOW()
		 WHERE email = $1 AND modo = 'cidadao'`,
		email,
	)
	return err
}

func (s *LimitesService) ObterUsoCidadao(email string) (map[string]interface{}, error) {
	if email == "" {
		return map[string]interface{}{
			"contagem": 0,
			"limite":   LimiteDiarioCidadao,
			"restante": LimiteDiarioCidadao,
			"plano":    "gratuito",
			"modo":     "cidadao",
		}, nil
	}

	var contagem int
	var ultimoUso time.Time

	err := s.DB.QueryRow(
		`SELECT contagem, ultimo_uso FROM uso_agente 
		 WHERE email = $1 AND modo = 'cidadao'`,
		email,
	).Scan(&contagem, &ultimoUso)

	if err == sql.ErrNoRows {
		return map[string]interface{}{
			"contagem": 0,
			"limite":   LimiteDiarioCidadao,
			"restante": LimiteDiarioCidadao,
			"plano":    "gratuito",
			"modo":     "cidadao",
		}, nil
	}

	if err != nil {
		return nil, err
	}

	// Se for outro dia, mostrar 0
	if ultimoUso.Format("2006-01-02") != time.Now().Format("2006-01-02") {
		contagem = 0
	}

	restante := LimiteDiarioCidadao - contagem
	if restante < 0 {
		restante = 0
	}

	return map[string]interface{}{
		"contagem":   contagem,
		"limite":     LimiteDiarioCidadao,
		"restante":   restante,
		"plano":      "gratuito",
		"modo":       "cidadao",
		"renova_em":  "meia-noite",
	}, nil
}

// ============================================
// PROFISSIONAL — 3 grátis total
// ============================================

func (s *LimitesService) PodePerguntar(email string, modo string) (bool, int, int, error) {
	if modo == "cidadao" {
		return s.PodePerguntarCidadao(email)
	}

	if email == "" {
		return true, 0, LimiteGratuitoProfissional, nil
	}

	var contagem int
	var plano string

	err := s.DB.QueryRow(
		`SELECT contagem, plano FROM uso_agente WHERE email = $1 AND modo = $2`,
		email, modo,
	).Scan(&contagem, &plano)

	if err == sql.ErrNoRows {
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

	if plano != "gratuito" {
		return true, contagem, -1, nil
	}

	pode := contagem < LimiteGratuitoProfissional
	return pode, contagem, LimiteGratuitoProfissional, nil
}

func (s *LimitesService) RegistarUso(email string, modo string) error {
	if email == "" {
		return nil
	}

	if modo == "cidadao" {
		return s.RegistarUsoCidadao(email)
	}

	_, err := s.DB.Exec(
		`UPDATE uso_agente 
		 SET contagem = contagem + 1, ultimo_uso = NOW()
		 WHERE email = $1 AND modo = $2`,
		email, modo,
	)
	return err
}

func (s *LimitesService) ObterUso(email string) (map[string]interface{}, error) {
	if email == "" {
		return map[string]interface{}{
			"contagem": 0,
			"limite":   LimiteGratuitoProfissional,
			"plano":    "gratuito",
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
		limite = -1
		restante = -1
	}

	if restante < 0 {
		restante = 0
	}

	return map[string]interface{}{
		"contagem":     contagem,
		"limite":       limite,
		"plano":        plano,
		"restante":     restante,
		"primeiro_uso": primeiro.Format(time.RFC3339),
		"ultimo_uso":   ultimo.Format(time.RFC3339),
	}, nil
}

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

func ValidarPlano(plano string) error {
	validos := map[string]bool{
		"gratuito":     true,
		"estudante":    true,
		"profissional": true,
		"empresarial":  true,
	}
	if !validos[plano] {
		return fmt.Errorf("plano inválido: %s", plano)
	}
	return nil
}
