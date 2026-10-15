package service

import (
	"fmt"
	"time"
)

// ContextoTemporal devolve a data/hora actual formatada para o prompt
func ContextoTemporal() string {
	agora := time.Now()
	dias := [...]string{
		"Domingo", "Segunda-feira", "Terça-feira",
		"Quarta-feira", "Quinta-feira", "Sexta-feira", "Sábado",
	}
	diaSemana := dias[int(agora.Weekday())]

	// Determinar período do dia
	hora := agora.Hour()
	var periodo string
	switch {
	case hora >= 5 && hora < 12:
		periodo = "manhã"
	case hora >= 12 && hora < 18:
		periodo = "tarde"
	case hora >= 18 && hora < 24:
		periodo = "noite"
	default:
		periodo = "madrugada"
	}

	return fmt.Sprintf(
		"DATA E HORA ACTUAL: %s, %s (%s) — hora de Moçambique (CAT, UTC+2)",
		diaSemana,
		agora.Format("02/01/2006 15:04"),
		periodo,
	)
}

// SaudacaoApropriada devolve a saudação correcta para a hora actual
func SaudacaoApropriada() string {
	hora := time.Now().Hour()
	switch {
	case hora >= 5 && hora < 12:
		return "Bom dia"
	case hora >= 12 && hora < 18:
		return "Boa tarde"
	default:
		return "Boa noite"
	}
}
