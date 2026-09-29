package handler

import (
	"encoding/json"
	"net/http"

	"github.com/wesleysemende133/buscador-juridico/internal/service"
)

type PlanosHandler struct {
	limites   *service.LimitesService
	validador *service.AuthValidator
}

func NewPlanosHandler(limites *service.LimitesService, validador *service.AuthValidator) *PlanosHandler {
	return &PlanosHandler{
		limites:   limites,
		validador: validador,
	}
}

// GET /planos - Lista os planos disponíveis
func (h *PlanosHandler) ListarPlanosHandler(w http.ResponseWriter, r *http.Request) {
	planos := []map[string]interface{}{
		{
			"id":          "gratuito",
			"nome":        "Gratuito",
			"preco":       0,
			"preco_texto": "Grátis",
			"periodo":     "para sempre",
			"descricao":   "Para começar a explorar",
			"features": []string{
				"3 perguntas ao Assistente IA Profissional",
				"Pesquisa ilimitada na legislação",
				"Acesso ao Modo Cidadão (ilimitado)",
				"Leitura de todos os artigos",
			},
			"limitacoes": []string{
				"Sem histórico de conversas",
				"Sem exportação PDF",
				"Sem acesso prioritário a advogados",
			},
			"destaque": false,
		},
		{
			"id":          "estudante",
			"nome":        "Estudante",
			"preco":       250,
			"preco_texto": "250 MZN",
			"periodo":     "/mês",
			"descricao":   "Para estudantes de Direito",
			"features": []string{
				"15 perguntas/mês ao Assistente IA Profissional",
				"Pesquisa ilimitada na legislação",
				"Acesso ao Modo Cidadão (ilimitado)",
				"Histórico de pesquisas",
				"Suporte por email",
			},
			"limitacoes": []string{
				"Sem exportação PDF",
			},
			"destaque": false,
			"requer_verificacao": true,
		},
		{
			"id":          "profissional",
			"nome":        "Profissional",
			"preco":       750,
			"preco_texto": "750 MZN",
			"periodo":     "/mês",
			"descricao":   "Para advogados e juristas",
			"features": []string{
				"Perguntas ILIMITADAS ao Assistente IA Profissional",
				"Pesquisa ilimitada na legislação",
				"Acesso ao Modo Cidadão (ilimitado)",
				"Acesso prioritário a advogados parceiros",
				"Exportação de conversas (PDF)",
				"Análises técnicas aprofundadas",
				"Suporte prioritário",
			},
			"limitacoes": []string{},
			"destaque":   true,
		},
		{
			"id":          "empresarial",
			"nome":        "Empresarial",
			"preco":       2500,
			"preco_texto": "2.500 MZN",
			"periodo":     "/mês",
			"descricao":   "Para escritórios e empresas",
			"features": []string{
				"Tudo do plano Profissional",
				"Até 10 utilizadores",
				"Painel de administração",
				"Relatórios de uso",
				"Gestor de conta dedicado",
				"Formação da equipa",
				"API de integração",
			},
			"limitacoes": []string{},
			"destaque":   false,
		},
	}

	respJSON(w, http.StatusOK, map[string]interface{}{
		"planos": planos,
		"total":  len(planos),
	})
}

// GET /planos/uso?email=...
func (h *PlanosHandler) UsoHandler(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")

	if email == "" {
		respErro(w, http.StatusUnauthorized, "precisas de criar conta")
		return
	}

	if h.validador != nil {
		existe, err := h.validador.UtilizadorExiste(email)
		if err != nil || !existe {
			respErro(w, http.StatusUnauthorized, "utilizador não encontrado")
			return
		}
	}

	uso, err := h.limites.ObterUso(email)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao obter uso")
		return
	}

	respJSON(w, http.StatusOK, uso)
}

// POST /planos/actualizar
func (h *PlanosHandler) ActualizarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
		Plano string `json:"plano"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	if req.Email == "" {
		respErro(w, http.StatusUnauthorized, "precisas de criar conta antes de comprar um plano")
		return
	}

	if h.validador != nil {
		existe, err := h.validador.UtilizadorExiste(req.Email)
		if err != nil || !existe {
			respErro(w, http.StatusUnauthorized, "utilizador não encontrado")
			return
		}
	}

	if err := service.ValidarPlano(req.Plano); err != nil {
		respErro(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.limites.ActualizarPlano(req.Email, req.Plano); err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao actualizar plano")
		return
	}

	respJSON(w, http.StatusOK, map[string]string{
		"mensagem": "Plano actualizado com sucesso",
		"plano":    req.Plano,
	})
}
