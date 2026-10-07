package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/wesleysemende133/buscador-juridico/internal/crawler"
	"github.com/wesleysemende133/buscador-juridico/internal/repository/postgres"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL não definida no .env")
	}

	pgRepo, err := postgres.NewPostgresRepository(dbURL)
	if err != nil {
		log.Fatalf("Erro ao ligar à BD: %v", err)
	}

	pastaPDFs := "data/pdfs"
	if len(os.Args) > 1 {
		pastaPDFs = os.Args[1]
	}

	crawler.InitRegexExtractor()

	files, err := filepath.Glob(filepath.Join(pastaPDFs, "*.pdf"))
	if err != nil {
		log.Fatal(err)
	}

	if len(files) == 0 {
		fmt.Printf("⚠️  Nenhum PDF encontrado em %s\n", pastaPDFs)
		return
	}

	fmt.Printf("📂 %d PDFs encontrados em %s\n\n", len(files), pastaPDFs)

	totalArtigos := 0
	totalAdicionados := 0
	ficheirosOK := 0
	ficheirosFalha := 0

	for i, file := range files {
		nome := filepath.Base(file)
		fmt.Printf("[%d/%d] 📄 %s\n", i+1, len(files), nome)

		dados, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("   ❌ Erro ao ler: %v\n", err)
			ficheirosFalha++
			continue
		}

		texto, err := crawler.ExtrairTextoPDFPublico(dados)
		if err != nil {
			fmt.Printf("   ❌ Erro ao extrair texto: %v\n", err)
			ficheirosFalha++
			continue
		}

		if len(texto) < 500 {
			fmt.Printf("   ⚠️  Texto muito curto (%d chars)\n", len(texto))
			ficheirosFalha++
			continue
		}

		artigos, err := crawler.ExtrairArtigosPublico(texto, nome)
		if err != nil {
			fmt.Printf("   ❌ Erro no regex: %v\n", err)
			ficheirosFalha++
			continue
		}

		if len(artigos) == 0 {
			fmt.Printf("   ⚠️  0 artigos extraídos\n")
			ficheirosFalha++
			continue
		}

		adicionados := 0
		for _, a := range artigos {
			if err := pgRepo.Adicionar(a); err == nil {
				adicionados++
			}
		}

		fmt.Printf("   ✅ %d extraídos, %d adicionados\n", len(artigos), adicionados)
		totalArtigos += len(artigos)
		totalAdicionados += adicionados
		ficheirosOK++
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("✅ RESUMO\n")
	fmt.Printf("========================================\n")
	fmt.Printf("📄 Ficheiros OK:         %d/%d\n", ficheirosOK, len(files))
	fmt.Printf("❌ Ficheiros com falha:  %d\n", ficheirosFalha)
	fmt.Printf("📝 Artigos extraídos:    %d\n", totalArtigos)
	fmt.Printf("➕ Artigos adicionados:  %d\n", totalAdicionados)

	count, _ := pgRepo.Contar()
	fmt.Printf("📊 Total na BD:          %d\n", count)
}
