package service

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
)

type DedupService struct {
	mu               sync.RWMutex
	idsExistentes    map[string]bool
	hashesExistentes map[string]string
}

func NewDedupService() *DedupService {
	return &DedupService{
		idsExistentes:    make(map[string]bool),
		hashesExistentes: make(map[string]string),
	}
}

// ============================================
// CARREGAR EXISTENTES (DETETA DUPLICADOS NO FICHEIRO)
// ============================================

func (d *DedupService) CarregarExistentes(artigos []domain.Artigo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	idsDuplicados := 0
	hashesDuplicados := 0

	for _, a := range artigos {
		// Verificar se ID já existe
		if d.idsExistentes[a.ID] {
			idsDuplicados++
			log.Printf("⚠️ Dedup: ID duplicado no ficheiro: %s", a.ID)
			continue // ignorar duplicado
		}
		d.idsExistentes[a.ID] = true

		// Verificar hash
		hash := d.calcularHash(a.Texto)
		if idExistente, existe := d.hashesExistentes[hash]; existe {
			hashesDuplicados++
			log.Printf("⚠️ Dedup: Texto duplicado: %s (já em %s)", a.ID, idExistente)
			continue
		}
		d.hashesExistentes[hash] = a.ID
	}

	log.Printf("📊 Dedup: %d IDs e %d hashes carregados", 
		len(d.idsExistentes), len(d.hashesExistentes))

	if idsDuplicados > 0 || hashesDuplicados > 0 {
		log.Printf("⚠️ Dedup: Encontrados %d IDs duplicados e %d textos duplicados",
			idsDuplicados, hashesDuplicados)
	}
}

// ============================================
// VERIFICAR SE É DUPLICADO
// ============================================

func (d *DedupService) IsDuplicado(artigo domain.Artigo) (bool, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// NÍVEL 1: Por ID
	if d.idsExistentes[artigo.ID] {
		return true, "ID duplicado: " + artigo.ID
	}

	// NÍVEL 2: Por hash do texto
	hash := d.calcularHash(artigo.Texto)
	if idExistente, existe := d.hashesExistentes[hash]; existe {
		return true, "Texto duplicado (já em " + idExistente + ")"
	}

	return false, ""
}

// ============================================
// ADICIONAR AO ÍNDICE
// ============================================

func (d *DedupService) Adicionar(artigo domain.Artigo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.idsExistentes[artigo.ID] = true
	hash := d.calcularHash(artigo.Texto)
	d.hashesExistentes[hash] = artigo.ID
}

// ============================================
// REMOVER DUPLICADOS DE UM LOTE
// ============================================

func (d *DedupService) RemoverDuplicados(artigos []domain.Artigo) ([]domain.Artigo, int) {
	var unicos []domain.Artigo
	duplicados := 0

	// Índices locais (para este lote)
	idsLocais := make(map[string]bool)
	hashesLocais := make(map[string]bool)

	for _, a := range artigos {
		// 1. Verificar contra existentes (globais)
		if duplicado, _ := d.IsDuplicado(a); duplicado {
			duplicados++
			continue
		}

		// 2. Verificar contra o mesmo lote
		if idsLocais[a.ID] {
			duplicados++
			continue
		}

		hash := d.calcularHash(a.Texto)
		if hashesLocais[hash] {
			duplicados++
			continue
		}

		// Não é duplicado
		idsLocais[a.ID] = true
		hashesLocais[hash] = true
		unicos = append(unicos, a)
	}

	return unicos, duplicados
}

// ============================================
// HASH DO TEXTO (NORMALIZADO)
// ============================================

func (d *DedupService) calcularHash(texto string) string {
	// Normalizar
	textoNorm := strings.ToLower(texto)
	textoNorm = regexp.MustCompile(`[^\w\s]`).ReplaceAllString(textoNorm, "")
	textoNorm = regexp.MustCompile(`\s+`).ReplaceAllString(textoNorm, " ")
	textoNorm = strings.TrimSpace(textoNorm)

	// SHA256
	hash := sha256.Sum256([]byte(textoNorm))
	return hex.EncodeToString(hash[:])
}

// ============================================
// ESTATÍSTICAS
// ============================================

func (d *DedupService) Estatisticas() (int, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.idsExistentes), len(d.hashesExistentes)
}
