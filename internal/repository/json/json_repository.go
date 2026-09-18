package json

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

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

// ============================================
// CARREGAR (ROBUSTO CONTRA FICHEIRO VAZIO)
// ============================================

func (r *JSONRepository) Carregar() ([]domain.Artigo, error) {
	data, err := os.ReadFile(r.caminho)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("📂 Arquivo %s não existe, criando vazio", r.caminho)
			// Criar ficheiro vazio
			os.WriteFile(r.caminho, []byte("[]"), 0644)
			return []domain.Artigo{}, nil
		}
		log.Printf("❌ Erro ao ler arquivo: %v", err)
		return nil, err
	}

	// PROTEÇÃO: se estiver vazio, retornar vazio
	if len(data) == 0 {
		log.Printf("📂 Arquivo %s está vazio, retornando vazio", r.caminho)
		return []domain.Artigo{}, nil
	}

	// PROTEÇÃO: se for só whitespace, retornar vazio
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		log.Printf("📂 Arquivo %s contém apenas whitespace/null, retornando vazio", r.caminho)
		return []domain.Artigo{}, nil
	}

	var artigos []domain.Artigo
	if err := json.Unmarshal(data, &artigos); err != nil {
		log.Printf("❌ Erro ao deserializar JSON: %v", err)
		return nil, err
	}

	if artigos == nil {
		artigos = []domain.Artigo{}
	}

	return artigos, nil
}

// ============================================
// SALVAR (ESCRITA ATÓMICA - NUNCA CORROMPE)
// ============================================

func (r *JSONRepository) Salvar(artigos []domain.Artigo) error {
	// PROTEÇÃO: nunca salvar nil (vira "null" no JSON)
	if artigos == nil {
		artigos = []domain.Artigo{}
	}

	log.Printf("💾 Salvando %d artigos em %s", len(artigos), r.caminho)

	data, err := json.MarshalIndent(artigos, "", "  ")
	if err != nil {
		log.Printf("❌ Erro ao serializar: %v", err)
		return err
	}

	// PROTEÇÃO: se serializou vazio, garantir que é "[]"
	if len(data) == 0 {
		data = []byte("[]")
	}

	// ESCRITA ATÓMICA: escreve num ficheiro temporário e depois renomeia
	// Isso evita corrupção se o processo for interrompido a meio
	tmpFile := r.caminho + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		log.Printf("❌ Erro ao escrever arquivo temporário: %v", err)
		return err
	}

	if err := os.Rename(tmpFile, r.caminho); err != nil {
		log.Printf("❌ Erro ao renomear arquivo: %v", err)
		// Tentar limpar o temporário
		os.Remove(tmpFile)
		return err
	}

	log.Printf("✅ Salvo com sucesso (%d bytes)", len(data))
	return nil
}

// ============================================
// BUSCAR POR ID
// ============================================

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

// ============================================
// LISTAR TODOS
// ============================================

func (r *JSONRepository) ListarTodos() ([]domain.Artigo, error) {
	return r.Carregar()
}

// ============================================
// BUSCAR POR TEXTO
// ============================================

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

// ============================================
// ADICIONAR (INDIVIDUAL)
// ============================================

func (r *JSONRepository) Adicionar(artigo domain.Artigo) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}

	// Verificar duplicado
	for _, a := range artigos {
		if a.ID == artigo.ID {
			return nil
		}
	}

	artigos = append(artigos, artigo)
	return r.Salvar(artigos)
}

// ============================================
// EDITAR
// ============================================

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

// ============================================
// EXCLUIR
// ============================================

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

// ============================================
// REVOGAR
// ============================================

func (r *JSONRepository) Revogar(id string, motivo string) error {
	artigos, err := r.Carregar()
	if err != nil {
		return err
	}
	for i, a := range artigos {
		if a.ID == id {
			artigos[i].Status = "Revogado"
			artigos[i].StatusMotivo = motivo
			artigos[i].AtualizadoEm = time.Now()
			return r.Salvar(artigos)
		}
	}
	return nil
}
