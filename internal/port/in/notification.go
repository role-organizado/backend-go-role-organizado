package in

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
)

// CreateNotificacaoInput holds fields to create a notification.
type CreateNotificacaoInput struct {
	UsuarioID string
	Tipo      domain.TipoNotificacao
	Titulo    string
	Mensagem  string
	Dados     map[string]string
}

// ListNotificacoesUseCase lists paginated notifications for a user.
type ListNotificacoesUseCase interface {
	Execute(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.Notificacao, int64, error)
}

// GetNotificacaoUseCase retrieves a single notification by ID.
type GetNotificacaoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*domain.Notificacao, error)
}

// CreateNotificacaoUseCase creates an in-app notification.
type CreateNotificacaoUseCase interface {
	Execute(ctx context.Context, in CreateNotificacaoInput) (*domain.Notificacao, error)
}

// MarcarLidaUseCase marks a single notification as read.
type MarcarLidaUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*domain.Notificacao, error)
}

// MarcarTodasLidasUseCase marks all user notifications as read.
type MarcarTodasLidasUseCase interface {
	Execute(ctx context.Context, usuarioID string) error
}

// DeleteNotificacaoUseCase deletes a notification.
type DeleteNotificacaoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// CountUnreadUseCase returns the unread count for a user.
type CountUnreadUseCase interface {
	Execute(ctx context.Context, usuarioID string) (int, error)
}
