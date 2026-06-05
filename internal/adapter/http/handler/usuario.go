package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UsuarioHandler handles HTTP requests for the Usuario resource.
type UsuarioHandler struct {
	getUsuario    portin.GetUsuarioUseCase
	updateUsuario portin.UpdateUsuarioUseCase
	listUsuarios  portin.ListUsuariosUseCase
	updateRole    portin.UpdateUserRoleUseCase
	mongo         *mongodb.Client
}

// NewUsuarioHandler creates a new UsuarioHandler.
func NewUsuarioHandler(
	get portin.GetUsuarioUseCase,
	update portin.UpdateUsuarioUseCase,
	list portin.ListUsuariosUseCase,
	updateRole portin.UpdateUserRoleUseCase,
	mongo *mongodb.Client,
) *UsuarioHandler {
	return &UsuarioHandler{
		getUsuario:    get,
		updateUsuario: update,
		listUsuarios:  list,
		updateRole:    updateRole,
		mongo:         mongo,
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
	r.Patch("/api/usuarios/{id}", h.UpdateByID)
	r.Get("/api/usuarios/{id}/devices", h.GetDevices)
}

// ---- DTOs ----

type usuarioDetailResponse struct {
	ID         string              `json:"id"`
	Nome       string              `json:"nome"`
	Email      string              `json:"email"`
	CPF        string              `json:"cpf,omitempty"`
	FotoPerfil string              `json:"fotoPerfil,omitempty"`
	Telefone   *telefoneReq        `json:"telefone,omitempty"`
	Endereco   *enderecoReq        `json:"endereco,omitempty"`
	Roles      []string            `json:"roles"`
	Ativo      bool                `json:"ativo"`
	CriadoEm  time.Time           `json:"criadoEm"`
	UpdatedAt  time.Time           `json:"updatedAt"`
}

type updateUsuarioRequest struct {
	Nome       string       `json:"nome"`
	Email      string       `json:"email"`
	CPF        string       `json:"cpf"`
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
	in := portin.UpdateUsuarioInput{Nome: req.Nome, Email: req.Email, CPF: req.CPF, FotoPerfil: req.FotoPerfil}
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
	in := portin.UpdateUsuarioInput{Nome: req.Nome, Email: req.Email, CPF: req.CPF, FotoPerfil: req.FotoPerfil}
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

// deviceResponse represents a registered push-notification device.
type deviceResponse struct {
	DeviceID string `json:"deviceId"`
	Platform string `json:"platform"`
	Token    string `json:"token"`
}

// GetDevices handles GET /api/usuarios/{id}/devices.
// Returns all push-notification devices registered for the given user.
// Returns an empty array if the collection is empty or does not exist.
func (h *UsuarioHandler) GetDevices(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")

	// Graceful degradation: if mongo is not wired (e.g. in tests), return empty array.
	if h.mongo == nil {
		writeJSON(w, http.StatusOK, []deviceResponse{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Try the most likely collection names used by Java.
	// Primary: "devices", fallback handled by returning empty array on error.
	col := h.mongo.Collection("devices")
	cursor, err := col.Find(ctx, bson.M{"usuario_id": userID})
	if err != nil {
		writeJSON(w, http.StatusOK, []deviceResponse{})
		return
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil || raw == nil {
		writeJSON(w, http.StatusOK, []deviceResponse{})
		return
	}

	devices := make([]deviceResponse, 0, len(raw))
	for _, d := range raw {
		dev := deviceResponse{}
		if v, ok := d["device_id"].(string); ok {
			dev.DeviceID = v
		} else if v, ok := d["deviceId"].(string); ok {
			dev.DeviceID = v
		}
		if v, ok := d["platform"].(string); ok {
			dev.Platform = v
		}
		if v, ok := d["token"].(string); ok {
			dev.Token = v
		}
		devices = append(devices, dev)
	}
	writeJSON(w, http.StatusOK, devices)
}

// ---- mapper ----

func toUsuarioDetailResponse(u auth.Usuario) usuarioDetailResponse {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	resp := usuarioDetailResponse{
		ID:        u.ID,
		Nome:      u.Nome,
		Email:     u.Email,
		CPF:       u.CPF,
		FotoPerfil: u.FotoPerfil,
		Roles:     roles,
		Ativo:     u.Ativo,
		CriadoEm: u.CriadoEm,
		UpdatedAt: u.UpdatedAt,
	}
	if u.Telefone != nil {
		resp.Telefone = &telefoneReq{DDI: u.Telefone.DDI, DDD: u.Telefone.DDD, Numero: u.Telefone.Numero, Tipo: u.Telefone.Tipo}
	}
	if u.Endereco != nil {
		resp.Endereco = &enderecoReq{
			Rua: u.Endereco.Rua, Numero: u.Endereco.Numero, Complemento: u.Endereco.Complemento,
			Bairro: u.Endereco.Bairro, Cidade: u.Endereco.Cidade, Estado: u.Endereco.Estado, CEP: u.Endereco.CEP,
		}
	}
	return resp
}
