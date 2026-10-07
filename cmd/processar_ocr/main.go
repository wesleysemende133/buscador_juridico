package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"

	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/repository/postgres"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL não definida")
	}

	pgRepo, err := postgres.NewPostgresRepository(dbURL)
	if err != nil {
		log.Fatalf("Erro: %v", err)
	}

	crawler.InitRegexExtractor()

	pastaOCR := "data/ocr"
	if len(os.Args) > 1 {
		pastaOCR = os.Args[1]
	}

	files, _ := filepath.Glob(filepath.Join(pastaOCR, "*.txt"))
	if len(files) == 0 {
		fmt.Printf("⚠️  Nenhum TXT em %s\n", pastaOCR)
		return
	}

	fmt.Printf("📂 %d TXT encontrados\n\n", len(files))

	total := 0
	adicionados := 0
	ficheirosOK := 0

	for i, file := range files {
		nome := filepath.Base(file)
		fmt.Printf("[%d/%d] 📄 %s\n", i+1, len(files), nome)

		dados, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("   ❌ %v\n", err)
			continue
		}

		texto := crawler.NormalizarTextoOCR(string(dados))

		if len(texto) < 500 {
			fmt.Printf("   ⚠️  Texto curto (%d chars)\n", len(texto))
			continue
		}

		artigos, err := crawler.ExtrairArtigosPublico(texto, strings.TrimSuffix(nome, ".txt"))
		if err != nil || len(artigos) == 0 {
			fmt.Printf("   ⚠️  0 artigos\n")
			continue
		}

		ad := 0
		for _, a := range artigos {
			if err := pgRepo.Adicionar(a); err == nil {
				ad++
			}
		}

		fmt.Printf("   ✅ %d extraídos, %d adicionados\n", len(artigos), ad)
		total += len(artigos)
		adicionados += ad
		ficheirosOK++
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("✅ RESUMO\n")
	fmt.Printf("========================================\n")
	fmt.Printf("📄 Ficheiros OK:        %d/%d\n", ficheirosOK, len(files))
	fmt.Printf("📝 Artigos extraídos:   %d\n", total)
	fmt.Printf("➕ Artigos adicionados: %d\n", adicionados)

	count, _ := pgRepo.Contar()
	fmt.Printf("📊 Total na BD:         %d\n", count)
}
