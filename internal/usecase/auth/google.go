package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// GoogleAuth implements portin.GoogleAuthUseCase.
type GoogleAuth struct {
	usuarios  portout.UsuarioRepository
	refreshes portout.RefreshTokenRepository
	jwtSvc    *jwt.Service
}

// NewGoogleAuth creates a new GoogleAuth use case.
func NewGoogleAuth(u portout.UsuarioRepository, rt portout.RefreshTokenRepository, j *jwt.Service) *GoogleAuth {
	return &GoogleAuth{usuarios: u, refreshes: rt, jwtSvc: j}
}

// googleTokenInfo is the minimal structure returned by Google's tokeninfo endpoint.
type googleTokenInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Picture string `json:"picture"`
}

// Execute validates the Google ID token and upserts the user.
func (uc *GoogleAuth) Execute(ctx context.Context, in portin.GoogleAuthInput) (*portin.AuthOutput, error) {
	slog.InfoContext(ctx, "google auth")

	info, err := verifyGoogleIDToken(ctx, in.IDToken)
	if err != nil {
		return nil, apierr.Unauthorized("token Google inválido: " + err.Error())
	}

	// Try to find existing user by provider ID first, then by email
	usuario, err := uc.usuarios.FindByProviderID(ctx, "google", info.Sub)
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}

	if usuario == nil {
		// Try by email
		usuario, err = uc.usuarios.FindByEmail(ctx, info.Email)
		if err != nil && !apierr.IsNotFound(err) {
			return nil, err
		}
	}

	if usuario == nil {
		// Create new user
		usuario = &domain.Usuario{
			Nome:       info.Name,
			Email:      info.Email,
			FotoPerfil: info.Picture,
			Ativo:      true,
			Roles:      []domain.Role{domain.RoleUser},
			ProviderLogin: []domain.ProviderLogin{
				{Provider: "google", ProviderUserID: info.Sub, Nome: info.Name, Email: info.Email, FotoPerfil: info.Picture},
			},
		}
		usuario, err = uc.usuarios.Save(ctx, usuario)
		if err != nil {
			return nil, err
		}
	} else {
		// Ensure the Google provider link is present
		found := false
		for _, p := range usuario.ProviderLogin {
			if p.Provider == "google" && p.ProviderUserID == info.Sub {
				found = true
				break
			}
		}
		if !found {
			usuario.ProviderLogin = append(usuario.ProviderLogin, domain.ProviderLogin{
				Provider: "google", ProviderUserID: info.Sub, Nome: info.Name, Email: info.Email, FotoPerfil: info.Picture,
			})
			usuario, err = uc.usuarios.Update(ctx, usuario)
			if err != nil {
				return nil, err
			}
		}
	}

	return issueTokens(ctx, usuario, uc.jwtSvc, uc.refreshes)
}

// verifyGoogleIDToken calls Google's tokeninfo endpoint.
func verifyGoogleIDToken(ctx context.Context, idToken string) (*googleTokenInfo, error) {
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + idToken
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling tokeninfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tokeninfo status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var info googleTokenInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parsing tokeninfo: %w", err)
	}
	if info.Sub == "" {
		return nil, fmt.Errorf("invalid tokeninfo: missing sub")
	}
	return &info, nil
}
