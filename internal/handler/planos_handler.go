package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/wesleysemende133/buscador-juridico/internal/auth"
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

// ============================================
// GET /planos — Público
// ============================================
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
				"20 perguntas/dia ao Assistente IA (Modo Cidadão)",
				"3 perguntas ao Assistente IA Profissional",
				"Pesquisa ilimitada na legislação",
				"Leitura de todos os artigos",
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
				"Histórico de pesquisas",
				"Suporte por email",
			},
			"destaque": false,
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
				"Acesso prioritário a advogados",
				"Exportação de conversas (PDF)",
				"Suporte prioritário",
			},
			"destaque": true,
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
				"Gestor de conta dedicado",
			},
			"destaque": false,
		},
	}

	respJSON(w, http.StatusOK, map[string]interface{}{
		"planos": planos,
		"total":  len(planos),
	})
}

// ============================================
// GET /planos/uso — Requer JWT
// Só o próprio utilizador pode ver o seu uso
// ============================================
func (h *PlanosHandler) UsoHandler(w http.ResponseWriter, r *http.Request) {
	// Email do JWT (não do query string)
	email, ok := r.Context().Value(auth.EmailKey).(string)
	if !ok || email == "" {
		respErro(w, http.StatusUnauthorized, "sessão inválida")
		return
	}

	uso, err := h.limites.ObterUso(email)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao obter uso")
		return
	}

	respJSON(w, http.StatusOK, uso)
}

// ============================================
// GET /planos/uso-cidadao — Requer JWT
// ============================================
func (h *PlanosHandler) UsoCidadaoHandler(w http.ResponseWriter, r *http.Request) {
	email, ok := r.Context().Value(auth.EmailKey).(string)
	if !ok || email == "" {
		respErro(w, http.StatusUnauthorized, "sessão inválida")
		return
	}

	uso, err := h.limites.ObterUsoCidadao(email)
	if err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao obter uso")
		return
	}

	respJSON(w, http.StatusOK, uso)
}

// ============================================
// POST /planos/actualizar — Requer JWT
//
// ⚠️  ATENÇÃO: este endpoint só funciona em desenvolvimento.
// Em produção, o plano é alterado via webhook do gateway de pagamento.
// ============================================
func (h *PlanosHandler) ActualizarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "método não permitido", http.StatusMethodNotAllowed)
		return
	}

	// ============================================
	// 1. BLOQUEIO EM PRODUÇÃO
	// ============================================
	ambiente := os.Getenv("AMBIENTE")
	if ambiente == "" {
		ambiente = "development"
	}

	if ambiente == "production" {
		respErro(w, http.StatusForbidden,
			"a alteração de planos em produção só pode ser feita via pagamento confirmado. "+
				"Contacta o suporte para mais informações.")
		return
	}

	// ============================================
	// 2. EXIGIR JWT
	// ============================================
	emailToken, ok := r.Context().Value(auth.EmailKey).(string)
	if !ok || emailToken == "" {
		respErro(w, http.StatusUnauthorized, "sessão inválida. Faz login novamente.")
		return
	}

	// ============================================
	// 3. VALIDAR QUE O UTILIZADOR EXISTE
	// ============================================
	if h.validador != nil {
		existe, err := h.validador.UtilizadorExiste(emailToken)
		if err != nil {
			respErro(w, http.StatusInternalServerError, "erro a validar utilizador")
			return
		}
		if !existe {
			respErro(w, http.StatusUnauthorized, "utilizador não encontrado")
			return
		}
	}

	// ============================================
	// 4. LER BODY (SEM email — vem do token)
	// ============================================
	var req struct {
		Plano string `json:"plano"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respErro(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	// ============================================
	// 5. VALIDAR PLANO
	// ============================================
	if err := service.ValidarPlano(req.Plano); err != nil {
		respErro(w, http.StatusBadRequest, err.Error())
		return
	}

	// ============================================
	// 6. ACTUALIZAR (usa email do TOKEN)
	// ============================================
	if err := h.limites.ActualizarPlano(emailToken, req.Plano); err != nil {
		respErro(w, http.StatusInternalServerError, "erro ao actualizar plano")
		return
	}

	respJSON(w, http.StatusOK, map[string]string{
		"mensagem": "Plano actualizado com sucesso",
		"plano":    req.Plano,
		"aviso":    "Endpoint de desenvolvimento — em produção será via pagamento",
	})
}
