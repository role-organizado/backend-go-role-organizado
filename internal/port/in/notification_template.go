package in

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
)

// --- Input DTOs ---

// CreateNotificationTemplateInput holds fields required to create a template.
type CreateNotificationTemplateInput struct {
	Nome               string
	Tipo               domain.TemplateType
	Categoria          domain.TemplateCategoria
	Assunto            string
	Corpo              string
	VariaveisEsperadas []string
	Ativo              bool
}

// UpdateNotificationTemplateInput holds updatable fields for a template.
type UpdateNotificationTemplateInput struct {
	ID                 string
	Nome               string
	Tipo               domain.TemplateType
	Categoria          domain.TemplateCategoria
	Assunto            string
	Corpo              string
	VariaveisEsperadas []string
	Ativo              bool
}

// TestSendInput carries the destination and variables for a test notification.
type TestSendInput struct {
	TemplateID  string
	Destinatario string            // email address, user ID, phone, etc.
	Variaveis   map[string]string
}

// --- Use Case Interfaces ---

// CreateNotificationTemplateUseCase creates a new notification template.
type CreateNotificationTemplateUseCase interface {
	Execute(ctx context.Context, in CreateNotificationTemplateInput) (*domain.NotificationTemplate, error)
}

// GetNotificationTemplateUseCase retrieves a single template by ID.
type GetNotificationTemplateUseCase interface {
	Execute(ctx context.Context, id string) (*domain.NotificationTemplate, error)
}

// ListNotificationTemplatesUseCase lists all templates with pagination.
type ListNotificationTemplatesUseCase interface {
	Execute(ctx context.Context, page, pageSize int) ([]domain.NotificationTemplate, int64, error)
}

// UpdateNotificationTemplateUseCase updates an existing template.
type UpdateNotificationTemplateUseCase interface {
	Execute(ctx context.Context, in UpdateNotificationTemplateInput) (*domain.NotificationTemplate, error)
}

// DeleteNotificationTemplateUseCase removes a template by ID.
type DeleteNotificationTemplateUseCase interface {
	Execute(ctx context.Context, id string) error
}

// RenderNotificationTemplateUseCase renders a template with variable substitution.
type RenderNotificationTemplateUseCase interface {
	Execute(ctx context.Context, id string, req domain.RenderRequest) (*domain.RenderResponse, error)
}

// TestSendNotificationTemplateUseCase renders and dispatches a test notification.
type TestSendNotificationTemplateUseCase interface {
	Execute(ctx context.Context, in TestSendInput) (*domain.RenderResponse, error)
}

// GetByTypeNotificationTemplateUseCase retrieves the first template matching a TemplateType.
type GetByTypeNotificationTemplateUseCase interface {
	Execute(ctx context.Context, tipo domain.TemplateType) (*domain.NotificationTemplate, error)
}

// ListByCategoriaNotificationTemplateUseCase lists templates filtered by TemplateCategoria.
type ListByCategoriaNotificationTemplateUseCase interface {
	Execute(ctx context.Context, categoria domain.TemplateCategoria, page, pageSize int) ([]domain.NotificationTemplate, int64, error)
}
