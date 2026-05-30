package auth

import (
	"context"
	"fmt"
	"log/slog"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
)

// RefreshToken implements portin.RefreshTokenUseCase.
type RefreshToken struct {
	refreshes portout.RefreshTokenRepository
	usuarios  portout.UsuarioRepository
	jwtSvc    *jwt.Service
}

// NewRefreshToken creates a new RefreshToken use case.
func NewRefreshToken(rt portout.RefreshTokenRepository, u portout.UsuarioRepository, j *jwt.Service) *RefreshToken {
	return &RefreshToken{refreshes: rt, usuarios: u, jwtSvc: j}
}

// Execute validates the refresh token, revokes it, and issues a new token pair.
func (uc *RefreshToken) Execute(ctx context.Context, refreshToken string) (*portin.AuthOutput, error) {
	slog.InfoContext(ctx, "refresh token")

	rtEntity, err := uc.refreshes.FindByToken(ctx, refreshToken)
	if err != nil {
		return nil, apierr.Unauthorized("refresh token inválido")
	}
	if rtEntity.IsExpired() || rtEntity.Used {
		return nil, apierr.Unauthorized("refresh token expirado")
	}

	// Revoke the used token
	if err := uc.refreshes.Revoke(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("revoking refresh token: %w", err)
	}

	usuario, err := uc.usuarios.FindByID(ctx, rtEntity.UsuarioID)
	if err != nil {
		return nil, apierr.Unauthorized("usuário não encontrado")
	}

	return issueTokens(ctx, usuario, uc.jwtSvc, uc.refreshes)
}

// ValidateToken implements portin.ValidateTokenUseCase.
type ValidateToken struct {
	usuarios portout.UsuarioRepository
	jwtSvc   *jwt.Service
}

// NewValidateToken creates a new ValidateToken use case.
func NewValidateToken(u portout.UsuarioRepository, j *jwt.Service) *ValidateToken {
	return &ValidateToken{usuarios: u, jwtSvc: j}
}

// Execute validates the JWT access token and returns the matching user.
func (uc *ValidateToken) Execute(ctx context.Context, accessToken string) (*portin.AuthOutput, error) {
	claims, err := uc.jwtSvc.ValidateToken(accessToken)
	if err != nil {
		return nil, apierr.Unauthorized("token inválido")
	}
	usuario, err := uc.usuarios.FindByID(ctx, claims.Sub)
	if err != nil {
		return nil, apierr.Unauthorized("usuário não encontrado")
	}
	return &portin.AuthOutput{Usuario: usuario, AccessToken: accessToken}, nil
}
