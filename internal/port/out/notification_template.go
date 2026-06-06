package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
)

// NotificationTemplateRepository is the persistence contract for notification templates.
type NotificationTemplateRepository interface {
	// CRUD
	Save(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error)
	FindByID(ctx context.Context, id string) (*domain.NotificationTemplate, error)
	FindAll(ctx context.Context, page, pageSize int) ([]domain.NotificationTemplate, int64, error)
	Update(ctx context.Context, t *domain.NotificationTemplate) (*domain.NotificationTemplate, error)
	DeleteByID(ctx context.Context, id string) error

	// Queries
	FindByType(ctx context.Context, tipo domain.TemplateType) (*domain.NotificationTemplate, error)
	FindByCategoria(ctx context.Context, categoria domain.TemplateCategoria, page, pageSize int) ([]domain.NotificationTemplate, int64, error)
}
