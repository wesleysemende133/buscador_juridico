package service

import (
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type BuscaService struct {
	buscador repository.Buscador
}

func NewBuscaService(buscador repository.Buscador) *BuscaService {
	return &BuscaService{buscador: buscador}
}

func (s *BuscaService) Buscar(query string, limite int) ([]domain.Artigo, error) {
	if limite <= 0 {
		limite = 10
	}
	artigos, err := s.buscador.BuscarPorTexto(query)
	if err != nil {
		return nil, err
	}
	if len(artigos) > limite {
		artigos = artigos[:limite]
	}
	return artigos, nil
}

func (s *BuscaService) BuscarPorID(id string) (*domain.Artigo, error) {
	return s.buscador.BuscarPorID(id)
}

func (s *BuscaService) ListarLeis() ([]string, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return nil, err
	}
	leisMap := make(map[string]bool)
	for _, a := range artigos {
		leisMap[a.Lei] = true
	}
	var leis []string
	for lei := range leisMap {
		leis = append(leis, lei)
	}
	return leis, nil
}
