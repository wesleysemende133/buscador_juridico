package service

import (
	"fmt"
	"time"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type AdminService struct {
	repo      repository.Repository
	buscador  repository.Buscador
	editor    repository.Editor
	idGen     infrastructure.IDGenerator
}

func NewAdminService(
	repo repository.Repository,
	buscador repository.Buscador,
	editor repository.Editor,
	idGen infrastructure.IDGenerator,
) *AdminService {
	return &AdminService{
		repo:     repo,
		buscador: buscador,
		editor:   editor,
		idGen:    idGen,
	}
}

func (s *AdminService) AdicionarArtigo(artigo domain.Artigo) error {
	artigo.ID = s.idGen.GenerateID()
	now := time.Now()
	artigo.CriadoEm = now
	artigo.AtualizadoEm = now
	artigo.Versao = 1

	if err := s.validarArtigo(artigo); err != nil {
		return err
	}
	return s.editor.Adicionar(artigo)
}

func (s *AdminService) EditarArtigo(id string, artigo domain.Artigo) error {
	existente, err := s.buscador.BuscarPorID(id)
	if err != nil {
		return err
	}
	if existente == nil {
		return fmt.Errorf("artigo com ID %s não encontrado", id)
	}

	artigo.ID = id
	artigo.CriadoEm = existente.CriadoEm
	artigo.Versao = existente.Versao + 1
	artigo.AtualizadoEm = time.Now()

	return s.editor.Editar(id, artigo)
}

func (s *AdminService) ExcluirArtigo(id string) error {
	return s.editor.Excluir(id)
}

func (s *AdminService) RevogarArtigo(id string, motivo string) error {
	return s.editor.Revogar(id, motivo)
}

func (s *AdminService) ListarArtigos() ([]domain.Artigo, error) {
	return s.buscador.ListarTodos()
}

func (s *AdminService) BuscarPorID(id string) (*domain.Artigo, error) {
	return s.buscador.BuscarPorID(id)
}

func (s *AdminService) validarArtigo(artigo domain.Artigo) error {
	if artigo.Lei == "" {
		return fmt.Errorf("lei é obrigatória")
	}
	if artigo.Artigo == "" {
		return fmt.Errorf("número do artigo é obrigatório")
	}
	if artigo.Texto == "" {
		return fmt.Errorf("texto do artigo é obrigatório")
	}
	return nil
}
