package service

import (
	"log"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type LimpezaService struct {
	repo     repository.Repository
	buscador repository.Buscador
}

func NewLimpezaService(repo repository.Repository, buscador repository.Buscador) *LimpezaService {
	return &LimpezaService{repo: repo, buscador: buscador}
}

// ============================================
// LIMPAR DUPLICADOS DO FICHEIRO
// ============================================

func (s *LimpezaService) LimparDuplicados() (int, int, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return 0, 0, err
	}

	totalAntes := len(artigos)

	vistos := make(map[string]bool)
	unicos := make([]domain.Artigo, 0, len(artigos))

	for _, a := range artigos {
		if vistos[a.ID] {
			continue
		}
		vistos[a.ID] = true
		unicos = append(unicos, a)
	}

	if len(unicos) < totalAntes {
		if err := s.repo.Salvar(unicos); err != nil {
			return 0, 0, err
		}
		log.Printf("🧹 Limpeza: %d → %d artigos (%d duplicados removidos)",
			totalAntes, len(unicos), totalAntes-len(unicos))
	} else {
		log.Println("✅ Nenhum duplicado encontrado")
	}

	return totalAntes, len(unicos), nil
}

// ============================================
// VERIFICAR DUPLICADOS (SEM REMOVER)
// ============================================

func (s *LimpezaService) VerificarDuplicados() (map[string]interface{}, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return nil, err
	}

	contagem := make(map[string]int)
	for _, a := range artigos {
		contagem[a.ID]++
	}

	var duplicados []map[string]interface{}
	for id, count := range contagem {
		if count > 1 {
			duplicados = append(duplicados, map[string]interface{}{
				"id":          id,
				"ocorrencias": count,
			})
		}
	}

	return map[string]interface{}{
		"total_artigos":    len(artigos),
		"total_unicos":     len(contagem),
		"total_duplicados": len(duplicados),
		"duplicados":       duplicados,
	}, nil
}
