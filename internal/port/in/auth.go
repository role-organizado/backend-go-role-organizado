package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
)

// LoginInput is the payload for email+password login.
type LoginInput struct {
	Email string
	Senha string
}

// RegisterInput is the payload for user registration.
type RegisterInput struct {
	Nome  string
	Email string
	Senha string
}

// GoogleAuthInput carries the Google ID token.
type GoogleAuthInput struct {
	IDToken string
}

// AppleAuthInput carries the Sign-in-with-Apple identity token.
type AppleAuthInput struct {
	IdentityToken string
	Nome          string // optional, sent on first login
}

// AuthOutput is returned by all auth use cases.
type AuthOutput struct {
	Usuario      *auth.Usuario
	AccessToken  string
	RefreshToken string
}

// LoginUseCase authenticates a user with email+password.
type LoginUseCase interface {
	Execute(ctx context.Context, in LoginInput) (*AuthOutput, error)
}

// RegisterUseCase creates a new user account.
type RegisterUseCase interface {
	Execute(ctx context.Context, in RegisterInput) (*AuthOutput, error)
}

// RefreshTokenUseCase rotates a refresh token and issues new tokens.
type RefreshTokenUseCase interface {
	Execute(ctx context.Context, refreshToken string) (*AuthOutput, error)
}

// ValidateTokenUseCase validates a JWT access token and returns the user.
type ValidateTokenUseCase interface {
	Execute(ctx context.Context, accessToken string) (*AuthOutput, error)
}

// LogoutUseCase revokes all refresh tokens for the authenticated user.
type LogoutUseCase interface {
	Execute(ctx context.Context, usuarioID string) error
}

// GoogleAuthUseCase authenticates or registers a user via Google Sign-In.
type GoogleAuthUseCase interface {
	Execute(ctx context.Context, in GoogleAuthInput) (*AuthOutput, error)
}

// AppleAuthUseCase authenticates or registers a user via Sign in with Apple.
type AppleAuthUseCase interface {
	Execute(ctx context.Context, in AppleAuthInput) (*AuthOutput, error)
}

// UpdateUsuarioInput is the payload for user profile updates.
// All fields are optional: scalar zero values are ignored; pointer fields
// are nil-skipped — this matches Java's UsuarioController.partialUpdate
// fetch-then-patch semantics.
type UpdateUsuarioInput struct {
	Nome       string
	Email      string
	CPF        string
	FotoPerfil string
	Telefone   *auth.Telefone
	Endereco   *auth.Endereco
	// AiMemoryOptOut, when non-nil, sets the user's AI memory opt-out flag.
	// nil means "do not change" (legacy partial-update semantics).
	AiMemoryOptOut *bool
}

// UpdateUsuarioUseCase updates an existing user profile.
type UpdateUsuarioUseCase interface {
	Execute(ctx context.Context, usuarioID string, in UpdateUsuarioInput) (*auth.Usuario, error)
}

// GetUsuarioUseCase returns a user by ID.
type GetUsuarioUseCase interface {
	Execute(ctx context.Context, id string) (*auth.Usuario, error)
}

// ListUsuariosUseCase lists users (admin).
type ListUsuariosUseCase interface {
	Execute(ctx context.Context, page, pageSize int) ([]auth.Usuario, int64, error)
}

// UpdateUserRoleInput carries the new roles for an admin role update.
type UpdateUserRoleInput struct {
	UsuarioID string
	Roles     []auth.Role
}

// UpdateUserRoleUseCase changes the roles of a user (admin only).
type UpdateUserRoleUseCase interface {
	Execute(ctx context.Context, in UpdateUserRoleInput) (*auth.Usuario, error)
}
