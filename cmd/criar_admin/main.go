package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL não definida no .env")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// ============================================
	// LER VARIÁVEIS DE AMBIENTE
	// ============================================
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@baselegal.mz"
	}

	senha := os.Getenv("ADMIN_PASSWORD")
	if senha == "" {
		senha = "admin123"
	}

	nome := os.Getenv("ADMIN_NOME")
	if nome == "" {
		nome = "Administrador"
	}

	// ============================================
	// VALIDAR
	// ============================================
	if len(senha) < 6 {
		log.Fatal("ADMIN_PASSWORD deve ter pelo menos 6 caracteres")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}

	// ============================================
	// INSERIR OU ACTUALIZAR
	// ============================================
	_, err = db.Exec(
		`INSERT INTO utilizadores (nome, email, senha_hash, role) 
		 VALUES ($1, $2, $3, 'admin')
		 ON CONFLICT (email) DO UPDATE SET 
		   senha_hash = EXCLUDED.senha_hash, 
		   role = 'admin',
		   nome = EXCLUDED.nome`,
		nome, email, string(hash),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ Admin criado/atualizado:")
	fmt.Printf("   Email: %s\n", email)
	fmt.Printf("   Senha: %s\n", senha)
	fmt.Printf("   Nome:  %s\n", nome)
	fmt.Println()
	fmt.Println("⚠️  MUDA A SENHA EM PRODUÇÃO!")
}
