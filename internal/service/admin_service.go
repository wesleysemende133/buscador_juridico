package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/infrastructure"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type AdminService struct {
	repo       repository.Repository
	buscador   repository.Buscador
	editor     repository.Editor
	idGen      infrastructure.IDGenerator
	dedup      *DedupService
	muSalvar   sync.Mutex
}

func NewAdminService(
	repo repository.Repository,
	buscador repository.Buscador,
	editor repository.Editor,
	idGen infrastructure.IDGenerator,
) *AdminService {
	s := &AdminService{
		repo:     repo,
		buscador: buscador,
		editor:   editor,
		idGen:    idGen,
		dedup:    NewDedupService(),
	}

	// Carregar artigos existentes no índice de dedup
	if artigos, err := buscador.ListarTodos(); err == nil {
		s.dedup.CarregarExistentes(artigos)
	}

	return s
}

// ============================================
// ADICIONAR VÁRIOS ARTIGOS (COM DEDUPLICAÇÃO)
// ============================================

func (s *AdminService) AdicionarVarios(artigos []domain.Artigo) (int, error) {
	if len(artigos) == 0 {
		return 0, nil
	}

	// Lock para evitar race conditions entre workers
	s.muSalvar.Lock()
	defer s.muSalvar.Unlock()

	log.Printf("📥 Recebidos %d artigos para processar", len(artigos))

	// ============================================
	// PASSO 1: DEDUPLICAÇÃO
	// ============================================
	unicos, duplicados := s.dedup.RemoverDuplicados(artigos)

	if duplicados > 0 {
		log.Printf("🔍 Dedup: %d artigos únicos, %d duplicados removidos", 
			len(unicos), duplicados)
	}

	if len(unicos) == 0 {
		log.Println("📊 Nenhum artigo novo para adicionar")
		return 0, nil
	}

	// ============================================
	// PASSO 2: CARREGAR EXISTENTES DO FICHEIRO
	// ============================================
	existentes, err := s.buscador.ListarTodos()
	if err != nil {
		return 0, fmt.Errorf("erro ao carregar existentes: %v", err)
	}

	// ============================================
	// PASSO 3: VALIDAR E ADICIONAR
	// ============================================
	adicionados := 0
	invalidos := 0
	now := time.Now()

	for _, artigo := range unicos {
		// Preencher metadados
		if artigo.Versao == 0 {
			artigo.Versao = 1
		}
		if artigo.CriadoEm.IsZero() {
			artigo.CriadoEm = now
		}
		if artigo.AtualizadoEm.IsZero() {
			artigo.AtualizadoEm = now
		}
		if artigo.Status == "" {
			artigo.Status = "Vigente"
		}

		// Validar
		if err := s.validarArtigo(artigo); err != nil {
			invalidos++
			continue
		}

		existentes = append(existentes, artigo)
		adicionados++
	}

	log.Printf("📊 Adicionados: %d | Duplicados: %d | Inválidos: %d",
		adicionados, duplicados, invalidos)

	// ============================================
	// PASSO 4: SALVAR
	// ============================================
	if adicionados > 0 {
		if err := s.repo.Salvar(existentes); err != nil {
			return 0, fmt.Errorf("erro ao salvar: %v", err)
		}

		// Atualizar índice de dedup com os novos
		for _, a := range unicos {
			s.dedup.Adicionar(a)
		}

		log.Printf("💾 Salvos %d artigos no total", len(existentes))
	}

	return adicionados, nil
}

// ============================================
// DEMAIS FUNÇÕES (mantém igual)
// ============================================

func (s *AdminService) AdicionarArtigo(artigo domain.Artigo) error {
	if artigo.ID == "" {
		artigo.ID = s.idGen.GenerateID()
	}

	now := time.Now()
	if artigo.CriadoEm.IsZero() {
		artigo.CriadoEm = now
	}
	if artigo.AtualizadoEm.IsZero() {
		artigo.AtualizadoEm = now
	}
	if artigo.Versao == 0 {
		artigo.Versao = 1
	}
	if artigo.Status == "" {
		artigo.Status = "Vigente"
	}

	if err := s.validarArtigo(artigo); err != nil {
		return err
	}

	// Verificar duplicado
	if duplicado, motivo := s.dedup.IsDuplicado(artigo); duplicado {
		return fmt.Errorf("artigo duplicado: %s", motivo)
	}

	err := s.editor.Adicionar(artigo)
	if err == nil {
		s.dedup.Adicionar(artigo)
	}
	return err
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

	if err := s.validarArtigo(artigo); err != nil {
		return err
	}

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

func (s *AdminService) ContarArtigos() (int, error) {
	artigos, err := s.buscador.ListarTodos()
	if err != nil {
		return 0, err
	}
	return len(artigos), nil
}

func (s *AdminService) validarArtigo(artigo domain.Artigo) error {
	if artigo.ID == "" {
		return fmt.Errorf("ID é obrigatório")
	}
	if artigo.Lei == "" {
		return fmt.Errorf("lei é obrigatória")
	}
	if artigo.Artigo == "" {
		return fmt.Errorf("número do artigo é obrigatório")
	}
	if artigo.Texto == "" {
		return fmt.Errorf("texto do artigo é obrigatório")
	}
	if len(artigo.Texto) < 20 {
		return fmt.Errorf("texto muito curto (%d caracteres)", len(artigo.Texto))
	}
	return nil
}

// ============================================
// ESTATÍSTICAS DE DEDUP
// ============================================

func (s *AdminService) EstatisticasDedup() (int, int) {
	return s.dedup.Estatisticas()
}
