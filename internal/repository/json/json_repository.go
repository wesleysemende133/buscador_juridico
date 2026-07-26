package json

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type JSONRepository struct {
	caminho string
}

var _ repository.Repository = (*JSONRepository)(nil)
var _ repository.Buscador = (*JSONRepository)(nil)
var _ repository.Editor = (*JSONRepository)(nil)

func NewJSONRepository(caminho string) *JSONRepository {
	return &JSONRepository{caminho: caminho}
}

func (r *JSONRepository) Carregar() ([]domain.Artigo, error) {
	data, err := os.ReadFile(r.caminho)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.Artigo{}, nil
		}
		return nil, err
	}
	var artigos []domain.Artigo
	if err := json.Unmarshal(data, &artigos); err != nil {
		return nil, err
	}
	return artigos, nil
}

func (r *JSONRepository) Salvar(artigos []domain.Artigo) error {
	data, err := json.MarshalIndent(artigos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.caminho, data, 0644)
}

func (r *JSONRepository) BuscarPorID(id string) (*domain.Artigo, error) {
	artigos, err := r.Carregar()
	if err != nil {
		return nil, err
	}
	for _, a := range artigos {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, nil
}

func (r *JSONRepository) ListarTodos() ([]domain.Artigo, error) {
	return r.Carregar()
}

func (r *JSONRepository) BuscarPorTexto(query string) ([]domain.Artigo, error) {
	artigos, err := r.Carregar()
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var resultados []domain.Artigo

	for _, a := range artigos {
		if strings.Contains(strings.ToLower(a.Texto), query) {
			resultados = append(resultados, a)
			continue
		}
		for _, p := range a.PalavrasChave {
			if strings.Contains(strings.ToLower(p), query) {
				resultados = append(resultados, a)
				break
			}
		}
	}
	return resultados, nil
}

func (r *JSONRepository) Adicionar(artigo domain.Artigo) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}
	artigos = append(artigos, artigo)
	return r.Salvar(artigos)
}

func (r *JSONRepository) Editar(id string, artigo domain.Artigo) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}
	for i, a := range artigos {
		if a.ID == id {
			artigos[i] = artigo
			return r.Salvar(artigos)
		}
	}
	return nil
}

func (r *JSONRepository) Excluir(id string) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}
	for i, a := range artigos {
		if a.ID == id {
			artigos = append(artigos[:i], artigos[i+1:]...)
			return r.Salvar(artigos)
		}
	}
	return nil
}

func (r *JSONRepository) Revogar(id string, motivo string) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}
	for i, a := range artigos {
		if a.ID == id {
			artigos[i].Status = "Revogado"
			artigos[i].StatusMotivo = motivo
			return r.Salvar(artigos)
		}
	}
	return nil
}
