// Package auth holds Identity & Auth domain use cases.
package auth

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/role-organizado/backend-go-role-organizado/pkg/jwt"
	"github.com/role-organizado/backend-go-role-organizado/pkg/tracing"
)

// Login implements portin.LoginUseCase.
type Login struct {
	usuarios  portout.UsuarioRepository
	refreshes portout.RefreshTokenRepository
	jwtSvc    *jwt.Service
}

// NewLogin creates a new Login use case.
func NewLogin(u portout.UsuarioRepository, rt portout.RefreshTokenRepository, j *jwt.Service) *Login {
	return &Login{usuarios: u, refreshes: rt, jwtSvc: j}
}

// Execute authenticates the user with email+password and returns a token pair.
func (uc *Login) Execute(ctx context.Context, in portin.LoginInput) (*portin.AuthOutput, error) {
	ctx, span := tracing.StartSpan(ctx, "usecase.auth.login")
	defer span.End()
	slog.InfoContext(ctx, "login attempt", "email", in.Email)

	usuario, err := uc.usuarios.FindByEmail(ctx, in.Email)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, apierr.Unauthorized("credenciais inválidas")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.SenhaHash), []byte(in.Senha)); err != nil {
		tracing.RecordError(span, err)
		return nil, apierr.Unauthorized("credenciais inválidas")
	}

	return issueTokens(ctx, usuario, uc.jwtSvc, uc.refreshes)
}

// Register implements portin.RegisterUseCase.
type Register struct {
	usuarios  portout.UsuarioRepository
	refreshes portout.RefreshTokenRepository
	jwtSvc    *jwt.Service
}

// NewRegister creates a new Register use case.
func NewRegister(u portout.UsuarioRepository, rt portout.RefreshTokenRepository, j *jwt.Service) *Register {
	return &Register{usuarios: u, refreshes: rt, jwtSvc: j}
}

// Execute creates a new user account and returns a token pair.
func (uc *Register) Execute(ctx context.Context, in portin.RegisterInput) (*portin.AuthOutput, error) {
	slog.InfoContext(ctx, "register attempt", "email", in.Email)

	if _, err := uc.usuarios.FindByEmail(ctx, in.Email); err == nil {
		return nil, apierr.Conflict("email já cadastrado")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Senha), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	u := &domain.Usuario{
		Nome:      in.Nome,
		Email:     in.Email,
		SenhaHash: string(hash),
		Ativo:     true,
		Roles:     []domain.Role{domain.RoleUser},
	}
	saved, err := uc.usuarios.Save(ctx, u)
	if err != nil {
		return nil, err
	}
	return issueTokens(ctx, saved, uc.jwtSvc, uc.refreshes)
}

// Logout implements portin.LogoutUseCase.
type Logout struct {
	refreshes portout.RefreshTokenRepository
}

// NewLogout creates a new Logout use case.
func NewLogout(rt portout.RefreshTokenRepository) *Logout {
	return &Logout{refreshes: rt}
}

// Execute revokes all refresh tokens for the user.
func (uc *Logout) Execute(ctx context.Context, usuarioID string) error {
	slog.InfoContext(ctx, "logout", "usuarioId", usuarioID)
	return uc.refreshes.RevokeAllForUser(ctx, usuarioID)
}

// issueTokens is a shared helper that creates a JWT token pair and persists the refresh token.
func issueTokens(ctx context.Context, u *domain.Usuario, j *jwt.Service, rt portout.RefreshTokenRepository) (*portin.AuthOutput, error) {
	pair, err := j.GenerateTokenPair(u.ID, u.Email, u.Nome, "", u.RoleStrings())
	if err != nil {
		return nil, fmt.Errorf("generating tokens: %w", err)
	}

	rtEntity := &domain.RefreshToken{
		UsuarioID: u.ID,
		Token:     pair.RefreshToken,
		ExpiresAt: pair.RefreshExpiresAt,
	}
	if _, err := rt.Save(ctx, rtEntity); err != nil {
		return nil, fmt.Errorf("saving refresh token: %w", err)
	}

	return &portin.AuthOutput{
		Usuario:      u,
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
	}, nil
}
