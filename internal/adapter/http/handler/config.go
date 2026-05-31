package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/config"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ConfigHandler handles HTTP requests for the Configuration domain.
type ConfigHandler struct {
	listDominios   portin.ListDominiosUseCase
	getDominio     portin.GetDominioUseCase
	upsertDominio  portin.UpsertDominioUseCase
	deleteDominio  portin.DeleteDominioUseCase
	getConfig      portin.GetConfigSistemaUseCase
	upsertConfig   portin.UpsertConfigSistemaUseCase
}

// NewConfigHandler creates a new ConfigHandler.
func NewConfigHandler(
	listDominios portin.ListDominiosUseCase,
	getDominio portin.GetDominioUseCase,
	upsertDominio portin.UpsertDominioUseCase,
	deleteDominio portin.DeleteDominioUseCase,
	getConfig portin.GetConfigSistemaUseCase,
	upsertConfig portin.UpsertConfigSistemaUseCase,
) *ConfigHandler {
	return &ConfigHandler{
		listDominios:  listDominios,
		getDominio:    getDominio,
		upsertDominio: upsertDominio,
		deleteDominio: deleteDominio,
		getConfig:     getConfig,
		upsertConfig:  upsertConfig,
	}
}

// RegisterRoutes mounts all Configuration domain routes on the given chi Router.
func (h *ConfigHandler) RegisterRoutes(r chi.Router) {
	// Public read endpoints
	r.Get("/api/v1/dominios", h.ListDominios)
	r.Get("/api/v1/dominios/{categoria}/{chave}", h.GetDominio)

	// UI theme config (authenticated, all roles)
	r.Get("/api/v1/config/ui-theme", h.GetUIThemeConfig)

	// Admin CRUD — require ADMIN role (guarded by RequireRole middleware applied in main.go)
	r.Get("/api/v1/admin/dominios", h.ListAllDominios) // Admin: lista todos incluindo inativos
	r.Post("/api/v1/admin/dominios", h.UpsertDominio)
	r.Put("/api/v1/admin/dominios/{id}", h.UpsertDominio)
	r.Delete("/api/v1/admin/dominios/{id}", h.DeleteDominio)
	r.Get("/api/v1/admin/config-sistema/{chave}", h.GetConfigSistema)
	r.Put("/api/v1/admin/config-sistema/{chave}", h.UpsertConfigSistema)
}

// ---- DTOs ----

// dominioResponse is the JSON representation sent to clients.
type dominioResponse struct {
	ID          string         `json:"id"`
	Categoria   string         `json:"categoria"`
	Chave       string         `json:"chave"`
	Valor       string         `json:"valor"`
	Descricao   string         `json:"descricao,omitempty"`
	Icone       string         `json:"icone,omitempty"`
	Ordem       int            `json:"ordem"`
	Ativo       bool           `json:"ativo"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CriadoEm    time.Time      `json:"criadoEm"`
	AtualizadoEm time.Time     `json:"atualizadoEm"`
}

type dominioUpsertRequest struct {
	Categoria string         `json:"categoria"`
	Chave     string         `json:"chave"`
	Valor     string         `json:"valor"`
	Descricao string         `json:"descricao"`
	Icone     string         `json:"icone"`
	Ordem     int            `json:"ordem"`
	Ativo     bool           `json:"ativo"`
	Metadata  map[string]any `json:"metadata"`
}

type configSistemaResponse struct {
	ID        string         `json:"id"`
	Chave     string         `json:"chave"`
	Valor     map[string]any `json:"valor"`
	Descricao string         `json:"descricao,omitempty"`
	Ativo     bool           `json:"ativo"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type configSistemaUpsertRequest struct {
	Valor     map[string]any `json:"valor"`
	Descricao string         `json:"descricao"`
	Ativo     bool           `json:"ativo"`
}

// ---- handlers ----

// ListDominios godoc
// @Summary Lista domínios configuráveis
// @Tags Domínios
// @Produce json
// @Param categoria query string false "Categoria"
// @Param ativo query bool false "Ativo"
// @Param tipoEvento query string false "Tipo de evento (Feature 008)"
// @Success 200 {array} dominioResponse
// @Router /api/v1/dominios [get]
func (h *ConfigHandler) ListDominios(w http.ResponseWriter, r *http.Request) {
	in := portin.ListDominiosInput{}

	if v := r.URL.Query().Get("categoria"); v != "" {
		in.Categoria = &v
	}
	if v := r.URL.Query().Get("ativo"); v != "" {
		b := strings.EqualFold(v, "true")
		in.Ativo = &b
	}
	if v := r.URL.Query().Get("tipoEvento"); v != "" {
		in.TipoEvento = &v
	}

	dominios, err := h.listDominios.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]dominioResponse, len(dominios))
	for i, d := range dominios {
		resp[i] = toDominioResponse(d)
	}
	writeJSON(w, http.StatusOK, resp)
}

// GetDominio godoc
// @Summary Busca domínio por categoria e chave
// @Tags Domínios
// @Produce json
// @Param categoria path string true "Categoria"
// @Param chave path string true "Chave"
// @Success 200 {object} dominioResponse
// @Router /api/v1/dominios/{categoria}/{chave} [get]
func (h *ConfigHandler) GetDominio(w http.ResponseWriter, r *http.Request) {
	categoria := chi.URLParam(r, "categoria")
	chave := chi.URLParam(r, "chave")

	d, err := h.getDominio.Execute(r.Context(), categoria, chave)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDominioResponse(*d))
}

// UpsertDominio handles POST /admin/dominios and PUT /admin/dominios/{id}.
func (h *ConfigHandler) UpsertDominio(w http.ResponseWriter, r *http.Request) {
	var req dominioUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}

	in := portin.UpsertDominioInput{
		Categoria: req.Categoria,
		Chave:     req.Chave,
		Valor:     req.Valor,
		Descricao: req.Descricao,
		Icone:     req.Icone,
		Ordem:     req.Ordem,
		Ativo:     req.Ativo,
		Metadata:  req.Metadata,
	}
	if id := chi.URLParam(r, "id"); id != "" {
		in.ID = &id
	}

	saved, err := h.upsertDominio.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDominioResponse(*saved))
}

// DeleteDominio handles DELETE /admin/dominios/{id}.
func (h *ConfigHandler) DeleteDominio(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.deleteDominio.Execute(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetConfigSistema handles GET /admin/config-sistema/{chave}.
func (h *ConfigHandler) GetConfigSistema(w http.ResponseWriter, r *http.Request) {
	chave := chi.URLParam(r, "chave")
	c, err := h.getConfig.Execute(r.Context(), chave)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toConfigSistemaResponse(*c))
}

// UpsertConfigSistema handles PUT /admin/config-sistema/{chave}.
func (h *ConfigHandler) UpsertConfigSistema(w http.ResponseWriter, r *http.Request) {
	chave := chi.URLParam(r, "chave")
	var req configSistemaUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	in := portin.UpsertConfigSistemaInput{
		Chave:     chave,
		Valor:     req.Valor,
		Descricao: req.Descricao,
		Ativo:     req.Ativo,
	}
	saved, err := h.upsertConfig.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toConfigSistemaResponse(*saved))
}

// GetUIThemeConfig handles GET /api/v1/config/ui-theme.
// Returns UI theme configuration — tries to fetch from DB, falls back to defaults.
func (h *ConfigHandler) GetUIThemeConfig(w http.ResponseWriter, r *http.Request) {
	c, err := h.getConfig.Execute(r.Context(), "UI_THEME_CONFIG")
	if err != nil {
		// Return default theme config if not found
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"chave": "UI_THEME_CONFIG",
			"valor": `{"primaryColor":"#0066ff","accentColor":"#8a2be2"}`,
			"ativo": true,
		})
		return
	}
	writeJSON(w, http.StatusOK, toConfigSistemaResponse(*c))
}

// ---- mappers ----

func toDominioResponse(d config.Dominio) dominioResponse {
	return dominioResponse{
		ID:        d.ID,
		Categoria: d.Categoria,
		Chave:     d.Chave,
		Valor:     d.Valor,
		Descricao: d.Descricao,
		Icone:     d.Icone,
		Ordem:     d.Ordem,
		Ativo:     d.Ativo,
		Metadata:  d.Metadata,
		CriadoEm:  d.CriadoEm,
		AtualizadoEm: d.UpdatedAt,
	}
}

// ListAllDominios handles GET /api/v1/admin/dominios — returns all dominios including inactive.
func (h *ConfigHandler) ListAllDominios(w http.ResponseWriter, r *http.Request) {
	// Reuse ListDominiosUseCase without ativo filter to return all records
	in := portin.ListDominiosInput{}
	if v := r.URL.Query().Get("categoria"); v != "" {
		in.Categoria = &v
	}
	dominios, err := h.listDominios.Execute(r.Context(), in)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]dominioResponse, len(dominios))
	for i, d := range dominios {
		resp[i] = toDominioResponse(d)
	}
	writeJSON(w, http.StatusOK, resp)
}

func toConfigSistemaResponse(c config.ConfiguracaoSistema) configSistemaResponse {
	return configSistemaResponse{
		ID:        c.ID,
		Chave:     c.Chave,
		Valor:     c.Valor,
		Descricao: c.Descricao,
		Ativo:     c.Ativo,
		UpdatedAt: c.UpdatedAt,
	}
}

// ---- shared HTTP helpers (used by all handlers in this package) ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	apiErr := apierr.From(err)
	writeJSON(w, apiErr.Status, apiErr)
}
