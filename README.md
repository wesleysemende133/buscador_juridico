```markdown
# ⚖️ Buscador Jurídico - Moçambique

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-MVP-green)]()

> **Motor de busca jurídica gratuito e público para Moçambique e PALOP**

---

## 📋 Sobre o Projeto

O **Buscador Jurídico** é uma API REST desenvolvida em Go que permite pesquisar, armazenar e gerenciar artigos de leis de Moçambique e dos Países Africanos de Língua Oficial Portuguesa (PALOP).

### 🎯 Objetivo

Democratizar o acesso à legislação, tornando a lei pública, acessível e pesquisável para todos os cidadãos, estudantes, advogados e instituições.

### 🔑 Princípios

- **Público e Gratuito** — A lei é pública e não pode ser cobrada
- **Acesso Democrático** — Qualquer cidadão pode pesquisar leis
- **Dados Oficiais** — Fontes confiáveis (Legis-PALOP+TL, Portais do Governo)
- **Arquitetura Limpa** — Código organizado, testável e escalável

---

## 🚀 Funcionalidades

### ✅ Já Implementadas

| Funcionalidade | Descrição |
| :--- | :--- |
| 🔍 **Busca por Palavra-Chave** | Encontre artigos por termo ou assunto |
| 📜 **Visualização de Artigos** | Veja o texto completo de qualquer artigo |
| 📚 **Listagem de Leis** | Consulte todas as leis disponíveis |
| ✏️ **CRUD Completo** | Adicione, edite e exclua artigos (admin) |
| 🕷️ **Crawler Automático** | Busca leis na internet (exemplo para Moçambique) |
| 🗂️ **Persistência em JSON** | Dados armazenados localmente (MVP) |
| 🏗️ **Arquitetura SOLID** | Código limpo, organizado e extensível |

### 🔜 Próximas Funcionalidades

- [ ] Banco de dados PostgreSQL/SQLite
- [ ] Histórico de versões dos artigos
- [ ] Autenticação e autorização (JWT)
- [ ] Busca semântica com IA
- [ ] Crawler real para Legis-PALOP+TL
- [ ] Front-end em Next.js
- [ ] Deploy na nuvem

---

## 🛠️ Tecnologias

| Camada | Tecnologia | Motivo |
| :--- | :--- | :--- |
| **Backend** | Go 1.26+ | Performance, concorrência, fácil deploy |
| **API** | `net/http` (padrão) | Simples, nativa, sem dependências |
| **Persistência** | JSON (atual) / PostgreSQL (futuro) | MVP rápido, migração fácil |
| **Crawler** | `gocolly/colly` | Melhor biblioteca de scraping em Go |
| **Arquitetura** | SOLID + Clean Architecture | Código sustentável e testável |

---

## 📂 Estrutura do Projeto

📂 ESTRUTURA DO PROJETO - BUSCADOR JURÍDICO

buscador-juridico/
│
├── cmd/
│   └── api/
│       └── main.go                    # Ponto de entrada da aplicação
│
├── internal/
│   ├── domain/                        # Entidades de negócio
│   │   └── artigo.go                  # Estrutura do Artigo
│   │
│   ├── repository/                    # Camada de persistência
│   │   ├── interfaces.go              # Interfaces (Repository, Buscador, Editor)
│   │   └── json/
│   │       └── json_repository.go     # Implementação JSON
│   │
│   ├── service/                       # Casos de uso (lógica de negócio)
│   │   ├── admin_service.go           # CRUD de artigos
│   │   └── busca_service.go           # Busca de artigos
│   │
│   ├── handler/                       # Controllers HTTP
│   │   ├── admin_handler.go           # Rotas administrativas
│   │   ├── busca_handler.go           # Rotas públicas
│   │   └── crawler_handler.go         # Rota do crawler
│   │
│   ├── crawler/                       # Scraping da internet
│   │   └── crawler.go                 # Busca automática de leis
│   │
│   └── infrastructure/                # Utilitários
│       └── id_generator.go            # Gerador de IDs únicos
│
├── data/
│   └── leis.json                      # Base de dados (MVP)
│
├── go.mod                             # Dependências do módulo
├── go.sum                             # Checksum das dependências
└── README.md                          # Documentação do projeto


📋 DESCRIÇÃO DAS PASTAS

┌─────────────────────────┬────────────────────────────────────────────────┐
│ Pasta                   │ Descrição                                      │
├─────────────────────────┼────────────────────────────────────────────────┤
│ cmd/api/                │ Ponto de entrada da aplicação (main.go)       │
│ internal/domain/        │ Entidades de negócio (Artigo, Historico)      │
│ internal/repository/    │ Interfaces e implementações de persistência    │
│ internal/service/       │ Casos de uso (lógica de negócio)              │
│ internal/handler/       │ Controllers HTTP (rotas e handlers)            │
│ internal/crawler/       │ Scraping da internet (busca automática)        │
│ internal/infrastructure/│ Utilitários (gerador de IDs, timestamps)       │
│ data/                   │ Arquivos de dados (leis.json)                  │
└─────────────────────────┴────────────────────────────────────────────────┘


📦 ARQUITETURA SOLID APLICADA

┌─────────────────────────────────────────────────────────────────────────────┐
│ PRINCÍPIO          │ APLICAÇÃO NO PROJETO                                 │
├────────────────────┼───────────────────────────────────────────────────────┤
│ S - SRP            │ Cada pacote tem uma responsabilidade única           │
│ O - OCP            │ Repository é interface → extensível sem modificar   │
│ L - LSP            │ Qualquer Repository pode substituir outro            │
│ I - ISP            │ Interfaces separadas: Repository, Buscador, Editor   │
│ D - DIP            │ Services dependem de interfaces, não de concretos    │
└────────────────────┴───────────────────────────────────────────────────────┘


🔗 ENDPOINTS DA API

PÚBLICOS (sem autenticação):
  GET  /buscar?q=termo          → Busca artigos por palavra-chave
  GET  /artigo/{id}             → Detalhe de um artigo específico
  GET  /leis                    → Lista todas as leis disponíveis

ADMIN (com autenticação - em desenvolvimento):
  GET  /admin/artigos           → Lista todos os artigos
  POST /admin/artigo            → Adiciona um novo artigo
  PUT  /admin/artigo/{id}       → Edita um artigo existente
  DELETE /admin/artigo/delete/{id} → Exclui um artigo
  GET  /crawler/buscar          → Executa o crawler (busca na internet)---

## 📦 Instalação

### Pré-requisitos

- Go 1.26+ ([Download](https://go.dev/dl/))

### Passos

```bash
# 1. Clone o repositório
git clone https://github.com/wesleysemende133/buscador-juridico.git
cd buscador-juridico

# 2. Baixe as dependências
go mod tidy

# 3. Rode o servidor
go run cmd/api/main.go
```

---

## 🔧 Configuração

### Variáveis de Ambiente

```bash
# Opcional: configurar a porta do servidor (padrão: 8080)
export PORT=8080
```

### Arquivo de Dados

O sistema usa `data/leis.json` como banco de dados inicial. Você pode:

- Adicionar artigos manualmente no arquivo
- Usar a API para adicionar (POST /admin/artigo)
- Executar o crawler para buscar automaticamente

---

## 📡 Endpoints da API

### 🔓 Públicos (sem autenticação)

| Método | Endpoint | Descrição |
| :--- | :--- | :--- |
| `GET` | `/buscar?q=termo&limite=10` | Busca artigos por palavra-chave |
| `GET` | `/artigo/{id}` | Detalhe de um artigo específico |
| `GET` | `/leis` | Lista todas as leis disponíveis |

### 🔒 Admin (com autenticação - em desenvolvimento)

| Método | Endpoint | Descrição |
| :--- | :--- | :--- |
| `GET` | `/admin/artigos` | Lista todos os artigos |
| `POST` | `/admin/artigo` | Adiciona um novo artigo |
| `PUT` | `/admin/artigo/{id}` | Edita um artigo existente |
| `DELETE` | `/admin/artigo/delete/{id}` | Exclui um artigo |
| `GET` | `/crawler/buscar` | Executa o crawler (busca na internet) |

---

## 🧪 Exemplos de Uso

### Buscar artigos sobre "guarda"

```bash
curl "http://localhost:8080/buscar?q=guarda"
```

### Listar todas as leis

```bash
curl "http://localhost:8080/leis"
```

### Adicionar um novo artigo (admin)

```bash
curl -X POST "http://localhost:8080/admin/artigo" \
  -H "Content-Type: application/json" \
  -d '{
    "lei": "Constituição da República de Moçambique",
    "lei_numero": "Constituição 2004",
    "artigo": "Art. 67",
    "texto": "A família é o elemento fundamental da sociedade...",
    "palavras_chave": ["família", "moçambique"],
    "status": "Vigente",
    "fonte": "Legis-PALOP+TL"
  }'
```

### Executar o crawler

```bash
curl "http://localhost:8080/crawler/buscar"
```

---

## 🧪 Testes

```bash
# Executar todos os testes
go test ./...

# Com cobertura
go test -cover ./...
```

---

## 🚀 Deploy

### Compilar para produção

```bash
# Compila para um binário único
go build -o buscador cmd/api/main.go

# Executa o binário
./buscador
```

### Docker (em breve)

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o buscador cmd/api/main.go

FROM alpine:latest
COPY --from=builder /app/buscador /buscador
EXPOSE 8080
CMD ["/buscador"]
```

---

## 🤝 Como Contribuir

1. Faça um fork do projeto
2. Crie uma branch para sua feature (`git checkout -b feature/nova-funcionalidade`)
3. Commit suas mudanças (`git commit -m 'Adiciona nova funcionalidade'`)
4. Push para a branch (`git push origin feature/nova-funcionalidade`)
5. Abra um Pull Request

---

## 📄 Licença

Este projeto está sob a licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.

---

## 📞 Contato

- **Autor:** Wesley Semende
- **Email:** [wesleysemende@gmail.com](mailto:wesleysemende@gmail.com)
- **GitHub:** [@wesleysemende133](https://github.com/wesleysemende133)

---

## 🙏 Agradecimentos

- [Legis-PALOP+TL](https://www.legis-palop.org/) — Fonte oficial de legislação
- [Go](https://go.dev/) — Linguagem que torna tudo possível
- [Colly](http://go-colly.org/) — Biblioteca de scraping

---

## ⭐️ Se você gostou deste projeto, deixe uma estrela!

---

**Made with ❤️ in Mozambique** 🇲🇿
