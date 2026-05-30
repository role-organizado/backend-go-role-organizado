package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
)

// UsuarioRepository is the output port for Usuario persistence.
type UsuarioRepository interface {
	FindByID(ctx context.Context, id string) (*auth.Usuario, error)
	FindByEmail(ctx context.Context, email string) (*auth.Usuario, error)
	FindByProviderID(ctx context.Context, provider, providerUserID string) (*auth.Usuario, error)
	Save(ctx context.Context, u *auth.Usuario) (*auth.Usuario, error)
	Update(ctx context.Context, u *auth.Usuario) (*auth.Usuario, error)
	FindAll(ctx context.Context, page, pageSize int) ([]auth.Usuario, int64, error)
	DeleteByID(ctx context.Context, id string) error
}

// RefreshTokenRepository is the output port for RefreshToken persistence.
type RefreshTokenRepository interface {
	Save(ctx context.Context, rt *auth.RefreshToken) (*auth.RefreshToken, error)
	FindByToken(ctx context.Context, token string) (*auth.RefreshToken, error)
	Revoke(ctx context.Context, token string) error
	RevokeAllForUser(ctx context.Context, usuarioID string) error
}
