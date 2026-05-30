package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
)

// NotificacaoRepository is the persistence contract for notifications.
type NotificacaoRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Notificacao, error)
	FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Notificacao, int64, error)
	FindUnreadByUsuarioID(ctx context.Context, usuarioID string) ([]domain.Notificacao, error)
	Save(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error)
	Update(ctx context.Context, n *domain.Notificacao) (*domain.Notificacao, error)
	MarkAllRead(ctx context.Context, usuarioID string) error
	DeleteByID(ctx context.Context, id string) error
}
