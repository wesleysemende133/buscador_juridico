package repository

import "github.com/wesleysemende133/buscador-juridico/internal/domain"

type Repository interface {
	Carregar() ([]domain.Artigo, error)
	Salvar([]domain.Artigo) error
}

type Buscador interface {
	BuscarPorID(id string) (*domain.Artigo, error)
	ListarTodos() ([]domain.Artigo, error)
	BuscarPorTexto(query string) ([]domain.Artigo, error)
}

type Editor interface {
	Adicionar(artigo domain.Artigo) error
	Editar(id string, artigo domain.Artigo) error
	Excluir(id string) error
	Revogar(id string, motivo string) error
}
