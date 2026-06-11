package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	pkgjwt "github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// BiometricHandler handles HTTP requests for the Biometric Auth domain.
// Public endpoints: challenge, authenticate, status.
// JWT-protected endpoints: register, list devices, revoke device.
type BiometricHandler struct {
	challengeUC    portin.GenerateBiometricChallengeUseCase
	authenticateUC portin.BiometricAuthenticateUseCase
	registerUC     portin.RegisterBiometricCredentialUseCase
	listDevicesUC  portin.ListBiometricDevicesUseCase
	revokeUC       portin.RevokeBiometricDeviceUseCase
	statusUC       portin.CheckBiometricStatusUseCase
}

// NewBiometricHandler wires the handler.
func NewBiometricHandler(
	challenge portin.GenerateBiometricChallengeUseCase,
	authenticate portin.BiometricAuthenticateUseCase,
	register portin.RegisterBiometricCredentialUseCase,
	listDevices portin.ListBiometricDevicesUseCase,
	revoke portin.RevokeBiometricDeviceUseCase,
	status portin.CheckBiometricStatusUseCase,
) *BiometricHandler {
	return &BiometricHandler{
		challengeUC:    challenge,
		authenticateUC: authenticate,
		registerUC:     register,
		listDevicesUC:  listDevices,
		revokeUC:       revoke,
		statusUC:       status,
	}
}

// RegisterPublicRoutes mounts the 3 unauthenticated endpoints (challenge / authenticate / status).
// Path prefix /api/auth/ is already in `publicPrefixes`, so these routes must NOT pass through
// the JWT middleware.
func (h *BiometricHandler) RegisterPublicRoutes(r chi.Router) {
	r.Post("/api/auth/biometric/challenge", h.challenge)
	r.Post("/api/auth/biometric/authenticate", h.authenticate)
	r.Get("/api/auth/biometric/status/{deviceId}", h.status)
}

// RegisterProtectedRoutes mounts the 3 JWT-protected endpoints under a route-local
// JWT middleware. We re-apply JWTAuth here because the parent middleware group
// skips /api/auth/* (a public prefix), and these specific routes still require auth.
func (h *BiometricHandler) RegisterProtectedRoutes(r chi.Router, jwtSvc *pkgjwt.Service) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(jwtSvc, nil))
		r.Post("/api/auth/biometric/register", h.register)
		r.Get("/api/auth/biometric/devices", h.listDevices)
		r.Delete("/api/auth/biometric/devices/{deviceId}", h.revoke)
	})
}

// ---- DTOs ----

type biometricChallengeRequest struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId"`
}

type biometricChallengeResponse struct {
	Challenge        string `json:"challenge"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

type biometricAuthenticateRequest struct {
	UserID    string `json:"userId"`
	DeviceID  string `json:"deviceId"`
	Signature string `json:"signature"`
}

type biometricRegisterRequest struct {
	DeviceID       string `json:"deviceId"`
	PublicKey      string `json:"publicKey"`
	DeviceName     string `json:"deviceName"`
	DeviceModel    string `json:"deviceModel"`
	AndroidVersion string `json:"androidVersion"`
}

type biometricRegisterResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	DeviceID     string `json:"deviceId"`
	DeviceName   string `json:"deviceName"`
	RegisteredAt string `json:"registeredAt"`
}

type biometricDeviceResponse struct {
	ID              string `json:"id"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	DeviceModel     string `json:"deviceModel,omitempty"`
	AndroidVersion  string `json:"androidVersion,omitempty"`
	RegisteredAt    string `json:"registeredAt"`
	LastUsedAt      string `json:"lastUsedAt,omitempty"`
	IsActive        bool   `json:"isActive"`
	IsCurrentDevice bool   `json:"isCurrentDevice"`
}

type biometricStatusResponse struct {
	DeviceID         string `json:"deviceId"`
	BiometricEnabled bool   `json:"biometricEnabled"`
}

type biometricRevokeResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	DeviceID string `json:"deviceId"`
}

// ---- Handlers ----

func (h *BiometricHandler) challenge(w http.ResponseWriter, r *http.Request) {
	var req biometricChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.challengeUC.Execute(r.Context(), portin.GenerateChallengeInput{
		UsuarioID: req.UserID,
		DeviceID:  req.DeviceID,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, biometricChallengeResponse{
		Challenge:        out.Challenge,
		ExpiresInSeconds: out.ExpiresInSeconds,
	})
}

func (h *BiometricHandler) authenticate(w http.ResponseWriter, r *http.Request) {
	var req biometricAuthenticateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.authenticateUC.Execute(
		r.Context(),
		portin.BiometricAuthenticateInput{
			UsuarioID: req.UserID,
			DeviceID:  req.DeviceID,
			Signature: req.Signature,
		},
		extractClientIP(r),
	)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, authResponse{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		Usuario:      toUsuarioResponse(*out.Usuario),
	})
}

func (h *BiometricHandler) status(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")
	enabled, err := h.statusUC.Execute(r.Context(), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, biometricStatusResponse{
		DeviceID:         deviceID,
		BiometricEnabled: enabled,
	})
}

func (h *BiometricHandler) register(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	var req biometricRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, apierr.BadRequest("corpo da requisição inválido"))
		return
	}
	out, err := h.registerUC.Execute(r.Context(), userID, portin.RegisterCredentialInput{
		DeviceID:       req.DeviceID,
		PublicKey:      req.PublicKey,
		DeviceName:     req.DeviceName,
		DeviceModel:    req.DeviceModel,
		AndroidVersion: req.AndroidVersion,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, biometricRegisterResponse{
		Success:      out.Success,
		Message:      out.Message,
		DeviceID:     out.DeviceID,
		DeviceName:   out.DeviceName,
		RegisteredAt: out.RegisteredAt,
	})
}

func (h *BiometricHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	currentDevice := r.URL.Query().Get("currentDeviceId")
	devices, err := h.listDevicesUC.Execute(r.Context(), userID, currentDevice)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]biometricDeviceResponse, len(devices))
	for i, d := range devices {
		out[i] = biometricDeviceResponse{
			ID:              d.ID,
			DeviceID:        d.DeviceID,
			DeviceName:      d.DeviceName,
			DeviceModel:     d.DeviceModel,
			AndroidVersion:  d.AndroidVersion,
			RegisteredAt:    d.RegisteredAt,
			LastUsedAt:      d.LastUsedAt,
			IsActive:        d.IsActive,
			IsCurrentDevice: d.IsCurrentDevice,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *BiometricHandler) revoke(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		writeError(w, apierr.Unauthorized("autenticação necessária"))
		return
	}
	deviceID := chi.URLParam(r, "deviceId")
	if err := h.revokeUC.Execute(r.Context(), userID, deviceID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, biometricRevokeResponse{
		Success:  true,
		Message:  "Dispositivo biométrico removido com sucesso",
		DeviceID: deviceID,
	})
}

// extractClientIP returns the best-effort caller IP — X-Forwarded-For (first),
// then X-Real-IP, then r.RemoteAddr. Mirrors Java's BiometricAuthController helper.
func extractClientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.IndexByte(v, ','); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

