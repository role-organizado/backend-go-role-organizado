package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// AuthHandler handles HTTP requests for the Auth domain.
type AuthHandler struct {
	login          portin.LoginUseCase
	register       portin.RegisterUseCase
	refresh        portin.RefreshTokenUseCase
	validate       portin.ValidateTokenUseCase
	logout         portin.LogoutUseCase
	googleAuth     portin.GoogleAuthUseCase
	appleAuth      portin.AppleAuthUseCase
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	login portin.LoginUseCase,
	register portin.RegisterUseCase,
	refresh portin.RefreshTokenUseCase,
	validate portin.ValidateTokenUseCase,
	logout portin.LogoutUseCase,
	googleAuth portin.GoogleAuthUseCase,
	appleAuth portin.AppleAuthUseCase,
) *AuthHandler {
	return &AuthHandler{
		login:      login,
		register:   register,
		refresh:    refresh,
		validate:   validate,
		logout:     logout,
		googleAuth: googleAuth,
		appleAuth:  appleAuth,
	}
}

// RegisterRoutes mounts all Auth routes on the given chi Router.
// Public routes go under /api/auth/* (unprotected prefix in JWT middleware).
// Validate and logout require a valid JWT (handled by middleware).
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/auth/refresh", h.Refresh)
	r.Get("/api/auth/validate", h.Validate)   // JWT required — validate endpoint returns user
	r.Post("/api/auth/logout", h.Logout)      // JWT required
	r.Post("/api/auth/google", h.GoogleAuth)
	r.Post("/api/auth/apple", h.AppleAuth)
	// Legacy prefix compatibility
	r.Post("/api/v1/auth/login", h.Login)
	r.Post("/api/v1/auth/register", h.Register)
	r.Post("/api/v1/auth/refresh", h.Refresh)
	r.Get("/api/v1/auth/validate", h.Validate)
	r.Post("/api/v1/auth/logout", h.Logout)
	r.Post("/api/v1/auth/google", h.GoogleAuth)
	r.Post("/api/v1/auth/apple", h.AppleAuth)
}

// ---- DTOs ----

type loginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type registerRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type googleAuthRequest struct {
	IDToken string `json:"idToken"`
}

type appleAuthRequest struct {
	IdentityToken string `json:"identityToken"`
	Nome          string `json:"nome"`
}

type authResponse struct {
	AccessToken  string         `json:"accessToken"`
	RefreshToken string         `json:"refreshToken,omitempty"`
	Usuario      usuarioResponse `json:"usuario"`
}

// ---- handlers ----

// Login godoc
// @Summary Login com email e senha
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Credenciais"
// @Success 200 {object} authResponse
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.login.Execute(r.Context(), portin.LoginInput{Email: req.Email, Senha: req.Senha})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(out))
}

// Register godoc
// @Summary Registrar novo usuário
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Dados do usuário"
// @Success 201 {object} authResponse
// @Router /api/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.register.Execute(r.Context(), portin.RegisterInput{Nome: req.Nome, Email: req.Email, Senha: req.Senha})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAuthResponse(out))
}

// Refresh godoc
// @Summary Renovar tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body refreshRequest true "Refresh token"
// @Success 200 {object} authResponse
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.refresh.Execute(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(out))
}

// Validate godoc
// @Summary Validar token JWT
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} authResponse
// @Router /api/auth/validate [get]
func (h *AuthHandler) Validate(w http.ResponseWriter, r *http.Request) {
	// JWT is already validated by middleware — extract token from header
	authHeader := r.Header.Get("Authorization")
	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}
	out, err := h.validate.Execute(r.Context(), token)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(out))
}

// Logout godoc
// @Summary Logout (revogar refresh tokens)
// @Tags Auth
// @Security BearerAuth
// @Success 204
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Logout is idempotent — return 204 even when not authenticated
	userID := middleware.UserIDFromContext(r.Context())
	if userID != "" {
		if err := h.logout.Execute(r.Context(), userID); err != nil {
			slog.WarnContext(r.Context(), "logout error", "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// GoogleAuth godoc
// @Summary Login com Google
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body googleAuthRequest true "Google ID token"
// @Success 200 {object} authResponse
// @Router /api/auth/google [post]
func (h *AuthHandler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
	var req googleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.googleAuth.Execute(r.Context(), portin.GoogleAuthInput{IDToken: req.IDToken})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(out))
}

// AppleAuth godoc
// @Summary Login com Apple
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body appleAuthRequest true "Apple identity token"
// @Success 200 {object} authResponse
// @Router /api/auth/apple [post]
func (h *AuthHandler) AppleAuth(w http.ResponseWriter, r *http.Request) {
	var req appleAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.appleAuth.Execute(r.Context(), portin.AppleAuthInput{IdentityToken: req.IdentityToken, Nome: req.Nome})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(out))
}

// ---- mappers ----

func toAuthResponse(out *portin.AuthOutput) authResponse {
	return authResponse{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		Usuario:      toUsuarioResponse(*out.Usuario),
	}
}

// ---- shared usuario response ----

type usuarioResponse struct {
	ID         string     `json:"id"`
	Nome       string     `json:"nome"`
	Email      string     `json:"email"`
	FotoPerfil string     `json:"fotoPerfil,omitempty"`
	Roles      []string   `json:"roles"`
	Ativo      bool       `json:"ativo"`
	CriadoEm  time.Time  `json:"criadoEm"`
}

func toUsuarioResponse(u auth.Usuario) usuarioResponse {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	return usuarioResponse{
		ID:         u.ID,
		Nome:       u.Nome,
		Email:      u.Email,
		FotoPerfil: u.FotoPerfil,
		Roles:      roles,
		Ativo:      u.Ativo,
		CriadoEm:  u.CriadoEm,
	}
}
