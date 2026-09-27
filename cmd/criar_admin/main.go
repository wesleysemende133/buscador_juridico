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

	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)

	_, err = db.Exec(
		`INSERT INTO utilizadores (nome, email, senha_hash, role) 
		 VALUES ($1, $2, $3, 'admin')
		 ON CONFLICT (email) DO UPDATE SET senha_hash = EXCLUDED.senha_hash`,
		"Administrador", "admin@baselegal.mz", string(hash),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ Admin criado/atualizado:")
	fmt.Println("   Email: admin@baselegal.mz")
	fmt.Println("   Senha: admin123")
	fmt.Println("   ⚠️  MUDA A SENHA EM PRODUÇÃO!")
}
