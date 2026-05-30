package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UsuarioHandler handles HTTP requests for the Usuario resource.
type UsuarioHandler struct {
	getUsuario     portin.GetUsuarioUseCase
	updateUsuario  portin.UpdateUsuarioUseCase
	listUsuarios   portin.ListUsuariosUseCase
	updateRole     portin.UpdateUserRoleUseCase
}

// NewUsuarioHandler creates a new UsuarioHandler.
func NewUsuarioHandler(
	get portin.GetUsuarioUseCase,
	update portin.UpdateUsuarioUseCase,
	list portin.ListUsuariosUseCase,
	updateRole portin.UpdateUserRoleUseCase,
) *UsuarioHandler {
	return &UsuarioHandler{
		getUsuario:    get,
		updateUsuario: update,
		listUsuarios:  list,
		updateRole:    updateRole,
	}
}

// RegisterRoutes mounts Usuario routes. JWT auth is enforced by middleware.
func (h *UsuarioHandler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/usuarios/me", h.Me)
	r.Put("/api/v1/usuarios/me", h.UpdateMe)
	r.Get("/api/v1/usuarios/{id}", h.GetByID)
	r.Get("/api/v1/admin/usuarios", h.ListAll)
	r.Put("/api/v1/admin/usuarios/{id}/roles", h.UpdateRole)

	// Legacy path aliases — same API surface as Java backend for BFF compatibility
	r.Get("/api/usuarios/{id}", h.GetByID)
	r.Put("/api/usuarios/{id}", h.UpdateByID)
}

// ---- DTOs ----

type usuarioDetailResponse struct {
	ID             string     `json:"id"`
	Nome           string     `json:"nome"`
	Email          string     `json:"email"`
	FotoPerfil     string     `json:"fotoPerfil,omitempty"`
	Roles          []string   `json:"roles"`
	Ativo          bool       `json:"ativo"`
	CriadoEm      time.Time  `json:"criadoEm"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type updateUsuarioRequest struct {
	Nome       string       `json:"nome"`
	FotoPerfil string       `json:"fotoPerfil"`
	Telefone   *telefoneReq `json:"telefone"`
	Endereco   *enderecoReq `json:"endereco"`
}

type telefoneReq struct {
	DDI    string `json:"ddi"`
	DDD    string `json:"ddd"`
	Numero string `json:"numero"`
	Tipo   string `json:"tipo"`
}

type enderecoReq struct {
	Rua         string `json:"rua"`
	Numero      string `json:"numero"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Cidade      string `json:"cidade"`
	Estado      string `json:"estado"`
	CEP         string `json:"cep"`
}

type updateRoleRequest struct {
	Roles []string `json:"roles"`
}

type paginatedUsuariosResponse struct {
	Content    []usuarioDetailResponse `json:"content"`
	Total      int64                   `json:"total"`
	Page       int                     `json:"page"`
	PageSize   int                     `json:"pageSize"`
}

// ---- handlers ----

// Me returns the authenticated user's profile.
func (h *UsuarioHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("não autenticado"))
		return
	}
	u, err := h.getUsuario.Execute(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUsuarioDetailResponse(*u))
}

// UpdateMe updates the authenticated user's profile.
func (h *UsuarioHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("não autenticado"))
		return
	}
	var req updateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	in := portin.UpdateUsuarioInput{Nome: req.Nome, FotoPerfil: req.FotoPerfil}
	if req.Telefone != nil {
		in.Telefone = &auth.Telefone{DDI: req.Telefone.DDI, DDD: req.Telefone.DDD, Numero: req.Telefone.Numero, Tipo: req.Telefone.Tipo}
	}
	if req.Endereco != nil {
		in.Endereco = &auth.Endereco{
			Rua: req.Endereco.Rua, Numero: req.Endereco.Numero, Complemento: req.Endereco.Complemento,
			Bairro: req.Endereco.Bairro, Cidade: req.Endereco.Cidade, Estado: req.Endereco.Estado, CEP: req.Endereco.CEP,
		}
	}
	u, err := h.updateUsuario.Execute(r.Context(), userID, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUsuarioDetailResponse(*u))
}

// GetByID returns a user by ID (admin or self).
func (h *UsuarioHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.getUsuario.Execute(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUsuarioDetailResponse(*u))
}

// UpdateByID updates a user by ID (legacy path — Java compat for BFF).
// Callers must own the resource; ownership is validated in the use case.
func (h *UsuarioHandler) UpdateByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	in := portin.UpdateUsuarioInput{Nome: req.Nome, FotoPerfil: req.FotoPerfil}
	if req.Telefone != nil {
		in.Telefone = &auth.Telefone{DDI: req.Telefone.DDI, DDD: req.Telefone.DDD, Numero: req.Telefone.Numero, Tipo: req.Telefone.Tipo}
	}
	if req.Endereco != nil {
		in.Endereco = &auth.Endereco{
			Rua: req.Endereco.Rua, Numero: req.Endereco.Numero, Complemento: req.Endereco.Complemento,
			Bairro: req.Endereco.Bairro, Cidade: req.Endereco.Cidade, Estado: req.Endereco.Estado, CEP: req.Endereco.CEP,
		}
	}
	u, err := h.updateUsuario.Execute(r.Context(), id, in)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUsuarioDetailResponse(*u))
}

// ListAll returns a paginated list of all users (admin only).
func (h *UsuarioHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	usuarios, total, err := h.listUsuarios.Execute(r.Context(), page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}
	content := make([]usuarioDetailResponse, len(usuarios))
	for i, u := range usuarios {
		content[i] = toUsuarioDetailResponse(u)
	}
	writeJSON(w, http.StatusOK, paginatedUsuariosResponse{Content: content, Total: total, Page: page, PageSize: pageSize})
}

// UpdateRole updates a user's roles (admin only).
func (h *UsuarioHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	roles := make([]auth.Role, len(req.Roles))
	for i, rr := range req.Roles {
		roles[i] = auth.Role(rr)
	}
	u, err := h.updateRole.Execute(r.Context(), portin.UpdateUserRoleInput{UsuarioID: id, Roles: roles})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUsuarioDetailResponse(*u))
}

// ---- mapper ----

func toUsuarioDetailResponse(u auth.Usuario) usuarioDetailResponse {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	return usuarioDetailResponse{
		ID:        u.ID,
		Nome:      u.Nome,
		Email:     u.Email,
		FotoPerfil: u.FotoPerfil,
		Roles:     roles,
		Ativo:     u.Ativo,
		CriadoEm: u.CriadoEm,
		UpdatedAt: u.UpdatedAt,
	}
}
