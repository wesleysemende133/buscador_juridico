# ============================================
# Base Legal — Makefile
# ============================================

.PHONY: help build run test clean criar-admin docker-up docker-down

help:
	@echo "Base Legal — Comandos disponíveis:"
	@echo ""
	@echo "  make build         Compilar o servidor"
	@echo "  make run           Correr o servidor"
	@echo "  make test          Correr testes"
	@echo "  make clean         Limpar binários"
	@echo "  make criar-admin   Criar admin inicial"
	@echo "  make docker-up     Subir Docker (PostgreSQL + Redis)"
	@echo "  make docker-down   Parar Docker"
	@echo ""

build:
	go build -o bin/servidor ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./... -v

clean:
	rm -rf bin/

criar-admin:
	go run ./cmd/criar_admin

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
