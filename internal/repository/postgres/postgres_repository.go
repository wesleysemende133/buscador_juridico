package postgres

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository"
)

type PostgresRepository struct {
	db *sql.DB
}

var _ repository.Repository = (*PostgresRepository)(nil)
var _ repository.Buscador = (*PostgresRepository)(nil)
var _ repository.Editor = (*PostgresRepository)(nil)

// ============================================
// CONSTRUTOR
// ============================================

func NewPostgresRepository(connStr string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erro ao conectar: %v", err)
	}

	log.Println("✅ Conectado ao PostgreSQL com sucesso")
	return &PostgresRepository{db: db}, nil
}

// ============================================
// REPOSITORY
// ============================================

func (r *PostgresRepository) Carregar() ([]domain.Artigo, error) {
	return r.ListarTodos()
}

func (r *PostgresRepository) Salvar(artigos []domain.Artigo) error {
	return nil
}

// ============================================
// BUSCADOR
// ============================================

func (r *PostgresRepository) ListarTodos() ([]domain.Artigo, error) {
	query := `
		SELECT id, lei, lei_numero, artigo, texto, palavras_chave,
		       categoria, subcategoria, versao, data_vigencia, data_publicacao,
		       status, status_motivo, fonte, url, criado_em, atualizado_em, aprovado_por
		FROM artigos
		ORDER BY lei_numero, id
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar: %v", err)
	}
	defer rows.Close()

	return r.scanArtigos(rows)
}

func (r *PostgresRepository) BuscarPorID(id string) (*domain.Artigo, error) {
	query := `
		SELECT id, lei, lei_numero, artigo, texto, palavras_chave,
		       categoria, subcategoria, versao, data_vigencia, data_publicacao,
		       status, status_motivo, fonte, url, criado_em, atualizado_em, aprovado_por
		FROM artigos
		WHERE id = $1
	`

	var a domain.Artigo
	var palavras pq.StringArray

	err := r.db.QueryRow(query, id).Scan(
		&a.ID, &a.Lei, &a.LeiNumero, &a.Artigo, &a.Texto, &palavras,
		&a.Categoria, &a.Subcategoria, &a.Versao, &a.DataVigencia, &a.DataPublicacao,
		&a.Status, &a.StatusMotivo, &a.Fonte, &a.URL, &a.CriadoEm, &a.AtualizadoEm, &a.AprovadoPor,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar: %v", err)
	}

	a.PalavrasChave = []string(palavras)
	return &a, nil
}

func (r *PostgresRepository) BuscarPorTexto(query string) ([]domain.Artigo, error) {
	sqlQuery := `
		SELECT id, lei, lei_numero, artigo, texto, palavras_chave,
		       categoria, subcategoria, versao, data_vigencia, data_publicacao,
		       status, status_motivo, fonte, url, criado_em, atualizado_em, aprovado_por
		FROM artigos
		WHERE 
			LOWER(texto) LIKE LOWER($1)
			OR LOWER(lei) LIKE LOWER($1)
			OR LOWER(artigo) LIKE LOWER($1)
			OR LOWER(categoria) LIKE LOWER($1)
			OR LOWER(subcategoria) LIKE LOWER($1)
		ORDER BY criado_em DESC
		LIMIT 100
	`

	rows, err := r.db.Query(sqlQuery, "%"+query+"%")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar: %v", err)
	}
	defer rows.Close()

	return r.scanArtigos(rows)
}

// ============================================
// EDITOR
// ============================================

func (r *PostgresRepository) Adicionar(artigo domain.Artigo) error {
	query := `
		INSERT INTO artigos (
			id, lei, lei_numero, artigo, texto, palavras_chave,
			categoria, subcategoria, versao, data_vigencia, data_publicacao,
			status, status_motivo, fonte, url, criado_em, atualizado_em, aprovado_por
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (id) DO NOTHING
	`

	// CORREÇÃO: usar pq.Array() em vez de JSON
	_, err := r.db.Exec(query,
		artigo.ID, artigo.Lei, artigo.LeiNumero, artigo.Artigo, artigo.Texto,
		pq.Array(artigo.PalavrasChave), // ← CORREÇÃO AQUI
		artigo.Categoria, artigo.Subcategoria, artigo.Versao,
		artigo.DataVigencia, artigo.DataPublicacao,
		artigo.Status, artigo.StatusMotivo, artigo.Fonte, artigo.URL,
		artigo.CriadoEm, artigo.AtualizadoEm, artigo.AprovadoPor,
	)

	if err != nil {
		return fmt.Errorf("erro ao adicionar: %v", err)
	}

	return nil
}

func (r *PostgresRepository) Editar(id string, artigo domain.Artigo) error {
	query := `
		UPDATE artigos SET
			lei = $1, lei_numero = $2, artigo = $3, texto = $4,
			palavras_chave = $5, categoria = $6, subcategoria = $7,
			versao = $8, data_vigencia = $9, status = $10,
			status_motivo = $11, fonte = $12, url = $13,
			atualizado_em = NOW(), aprovado_por = $14
		WHERE id = $15
	`

	// CORREÇÃO: usar pq.Array()
	_, err := r.db.Exec(query,
		artigo.Lei, artigo.LeiNumero, artigo.Artigo, artigo.Texto,
		pq.Array(artigo.PalavrasChave), // ← CORREÇÃO AQUI
		artigo.Categoria, artigo.Subcategoria,
		artigo.Versao, artigo.DataVigencia, artigo.Status,
		artigo.StatusMotivo, artigo.Fonte, artigo.URL,
		artigo.AprovadoPor, id,
	)

	return err
}

func (r *PostgresRepository) Excluir(id string) error {
	_, err := r.db.Exec("DELETE FROM artigos WHERE id = $1", id)
	return err
}

func (r *PostgresRepository) Revogar(id string, motivo string) error {
	_, err := r.db.Exec(
		"UPDATE artigos SET status = 'Revogado', status_motivo = $1, atualizado_em = NOW() WHERE id = $2",
		motivo, id,
	)
	return err
}

// ============================================
// HELPERS
// ============================================

func (r *PostgresRepository) scanArtigos(rows *sql.Rows) ([]domain.Artigo, error) {
	var artigos []domain.Artigo

	for rows.Next() {
		var a domain.Artigo
		var palavras pq.StringArray // ← CORREÇÃO

		err := rows.Scan(
			&a.ID, &a.Lei, &a.LeiNumero, &a.Artigo, &a.Texto, &palavras,
			&a.Categoria, &a.Subcategoria, &a.Versao, &a.DataVigencia, &a.DataPublicacao,
			&a.Status, &a.StatusMotivo, &a.Fonte, &a.URL, &a.CriadoEm, &a.AtualizadoEm, &a.AprovadoPor,
		)
		if err != nil {
			return nil, err
		}

		a.PalavrasChave = []string(palavras)
		artigos = append(artigos, a)
	}

	if artigos == nil {
		artigos = []domain.Artigo{}
	}

	return artigos, nil
}

// ============================================
// CONTAR
// ============================================

func (r *PostgresRepository) Contar() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM artigos").Scan(&count)
	return count, err
}

// ============================================
// BUSCA FULL-TEXT
// ============================================

func (r *PostgresRepository) BuscarFullText(query string, limite int) ([]domain.Artigo, error) {
	sqlQuery := `
		SELECT id, lei, lei_numero, artigo, texto, palavras_chave,
		       categoria, subcategoria, versao, data_vigencia, data_publicacao,
		       status, status_motivo, fonte, url, criado_em, atualizado_em, aprovado_por
		FROM artigos
		WHERE 
			to_tsvector('portuguese', texto) @@ plainto_tsquery('portuguese', $1)
			OR to_tsvector('portuguese', lei) @@ plainto_tsquery('portuguese', $1)
		ORDER BY ts_rank(to_tsvector('portuguese', texto), plainto_tsquery('portuguese', $1)) DESC
		LIMIT $2
	`

	rows, err := r.db.Query(sqlQuery, query, limite)
	if err != nil {
		return nil, fmt.Errorf("erro na busca full-text: %v", err)
	}
	defer rows.Close()

	return r.scanArtigos(rows)
}

// ============================================
// EXPOR LIGAÇÃO (para reutilizar noutros serviços)
// ============================================

func (r *PostgresRepository) DB() *sql.DB {
	return r.db
}