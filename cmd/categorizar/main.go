package main

import (
	"encoding/json"
	"log"
	"os"
	"sort"

	"github.com/wesleysemende133/buscador-juridico/internal/domain"
	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

func main() {
	caminho := "data/leis.json"

	data, err := os.ReadFile(caminho)
	if err != nil {
		log.Fatalf("Erro ao ler: %v", err)
	}

	var artigos []domain.Artigo
	if err := json.Unmarshal(data, &artigos); err != nil {
		log.Fatalf("Erro ao deserializar: %v", err)
	}

	log.Printf("📊 %d artigos carregados", len(artigos))

	categorizador := service.NewCategorizacaoService()

	for i := range artigos {
		// Normalizar nome da lei
		artigos[i].Lei = categorizador.NormalizarNomeLei(artigos[i].Lei, artigos[i].LeiNumero)
		// Categorizar
		categorizador.Categorizar(&artigos[i])
	}

	data, err = json.MarshalIndent(artigos, "", "  ")
	if err != nil {
		log.Fatalf("Erro ao serializar: %v", err)
	}
	if err := os.WriteFile(caminho, data, 0644); err != nil {
		log.Fatalf("Erro ao salvar: %v", err)
	}

	// Estatísticas
	contagem := make(map[string]int)
	for _, a := range artigos {
		contagem[a.Categoria]++
	}

	log.Println()
	log.Println("✅ Categorização concluída!")
	log.Println()
	log.Println("📊 Distribuição por categoria:")

	// Ordenar categorias
	var cats []string
	for cat := range contagem {
		cats = append(cats, cat)
	}
	sort.Slice(cats, func(i, j int) bool {
		return contagem[cats[i]] > contagem[cats[j]]
	})

	for _, cat := range cats {
		log.Printf("   %5d artigos  →  %s", contagem[cat], cat)
	}
}
