package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/repository/postgres"
)

func main() {
	log.Println("🔄 Iniciando migração JSON → PostgreSQL")

	// Carregar .env
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  Ficheiro .env não encontrado, usando variáveis de ambiente")
	}

	// 1. Ler artigos do JSON
	data, err := os.ReadFile("data/leis.json")
	if err != nil {
		log.Fatalf("❌ Erro ao ler JSON: %v", err)
	}

	var artigos []domain.Artigo
	if err := json.Unmarshal(data, &artigos); err != nil {
		log.Fatalf("❌ Erro ao deserializar JSON: %v", err)
	}

	log.Printf("📊 %d artigos carregados do JSON", len(artigos))

	// 2. Conectar ao PostgreSQL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("❌ DATABASE_URL não configurada no .env")
	}

	repo, err := postgres.NewPostgresRepository(dbURL)
	if err != nil {
		log.Fatalf("❌ Erro ao conectar: %v", err)
	}

	// 3. Inserir artigos
	log.Println("📥 Inserindo artigos no PostgreSQL...")

	sucesso := 0
	erros := 0

	for i, artigo := range artigos {
		if err := repo.Adicionar(artigo); err != nil {
			log.Printf("⚠️  Erro no artigo %d (%s): %v", i, artigo.ID, err)
			erros++
		} else {
			sucesso++
		}

		// Mostrar progresso a cada 100
		if (i+1)%100 == 0 {
			log.Printf("   Progresso: %d/%d", i+1, len(artigos))
		}
	}

	// 4. Verificar total
	total, _ := repo.Contar()

	log.Println()
	log.Printf("✅ Migração concluída!")
	log.Printf("   Sucesso: %d", sucesso)
	log.Printf("   Erros: %d", erros)
	log.Printf("   Total no PostgreSQL: %d", total)
}
