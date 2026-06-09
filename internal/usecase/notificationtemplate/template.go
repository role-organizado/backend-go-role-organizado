// Package notificationtemplate implements the 9 use cases for notification templates.
package notificationtemplate

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notificationtemplate"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- CreateNotificationTemplate ----

// CreateNotificationTemplate creates a new notification template.
type CreateNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewCreateNotificationTemplate creates a CreateNotificationTemplate use case.
func NewCreateNotificationTemplate(repo portout.NotificationTemplateRepository) *CreateNotificationTemplate {
	return &CreateNotificationTemplate{repo: repo}
}

func (uc *CreateNotificationTemplate) Execute(ctx context.Context, in portin.CreateNotificationTemplateInput) (*domain.NotificationTemplate, error) {
	if in.Nome == "" {
		return nil, apierr.BadRequest("nome é obrigatório")
	}
	if in.Tipo == "" {
		return nil, apierr.BadRequest("tipo é obrigatório")
	}
	if in.Corpo == "" {
		return nil, apierr.BadRequest("corpo é obrigatório")
	}

	now := time.Now()
	t := &domain.NotificationTemplate{
		Nome:               in.Nome,
		Tipo:               in.Tipo,
		Categoria:          in.Categoria,
		Assunto:            in.Assunto,
		Corpo:              in.Corpo,
		VariaveisEsperadas: in.VariaveisEsperadas,
		Ativo:              in.Ativo,
		CriadoEm:           now,
		AtualizadoEm:       now,
	}
	return uc.repo.Save(ctx, t)
}

var _ portin.CreateNotificationTemplateUseCase = (*CreateNotificationTemplate)(nil)

// ---- GetNotificationTemplate ----

// GetNotificationTemplate retrieves a notification template by ID.
type GetNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewGetNotificationTemplate creates a GetNotificationTemplate use case.
func NewGetNotificationTemplate(repo portout.NotificationTemplateRepository) *GetNotificationTemplate {
	return &GetNotificationTemplate{repo: repo}
}

func (uc *GetNotificationTemplate) Execute(ctx context.Context, id string) (*domain.NotificationTemplate, error) {
	return uc.repo.FindByID(ctx, id)
}

var _ portin.GetNotificationTemplateUseCase = (*GetNotificationTemplate)(nil)

// ---- ListNotificationTemplates ----

// ListNotificationTemplates lists all notification templates with pagination.
type ListNotificationTemplates struct {
	repo portout.NotificationTemplateRepository
}

// NewListNotificationTemplates creates a ListNotificationTemplates use case.
func NewListNotificationTemplates(repo portout.NotificationTemplateRepository) *ListNotificationTemplates {
	return &ListNotificationTemplates{repo: repo}
}

func (uc *ListNotificationTemplates) Execute(ctx context.Context, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.FindAll(ctx, page, pageSize)
}

var _ portin.ListNotificationTemplatesUseCase = (*ListNotificationTemplates)(nil)

// ---- UpdateNotificationTemplate ----

// UpdateNotificationTemplate updates an existing notification template.
type UpdateNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewUpdateNotificationTemplate creates an UpdateNotificationTemplate use case.
func NewUpdateNotificationTemplate(repo portout.NotificationTemplateRepository) *UpdateNotificationTemplate {
	return &UpdateNotificationTemplate{repo: repo}
}

func (uc *UpdateNotificationTemplate) Execute(ctx context.Context, in portin.UpdateNotificationTemplateInput) (*domain.NotificationTemplate, error) {
	existing, err := uc.repo.FindByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if in.Nome != "" {
		existing.Nome = in.Nome
	}
	if in.Tipo != "" {
		existing.Tipo = in.Tipo
	}
	if in.Categoria != "" {
		existing.Categoria = in.Categoria
	}
	if in.Assunto != "" {
		existing.Assunto = in.Assunto
	}
	if in.Corpo != "" {
		existing.Corpo = in.Corpo
	}
	if in.VariaveisEsperadas != nil {
		existing.VariaveisEsperadas = in.VariaveisEsperadas
	}
	existing.Ativo = in.Ativo
	existing.AtualizadoEm = time.Now()

	return uc.repo.Update(ctx, existing)
}

var _ portin.UpdateNotificationTemplateUseCase = (*UpdateNotificationTemplate)(nil)

// ---- DeleteNotificationTemplate ----

// DeleteNotificationTemplate removes a notification template by ID.
type DeleteNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewDeleteNotificationTemplate creates a DeleteNotificationTemplate use case.
func NewDeleteNotificationTemplate(repo portout.NotificationTemplateRepository) *DeleteNotificationTemplate {
	return &DeleteNotificationTemplate{repo: repo}
}

func (uc *DeleteNotificationTemplate) Execute(ctx context.Context, id string) error {
	// Verify existence before delete.
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.DeleteByID(ctx, id)
}

var _ portin.DeleteNotificationTemplateUseCase = (*DeleteNotificationTemplate)(nil)

// ---- RenderNotificationTemplate ----

// RenderNotificationTemplate renders a template by substituting variables.
type RenderNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewRenderNotificationTemplate creates a RenderNotificationTemplate use case.
func NewRenderNotificationTemplate(repo portout.NotificationTemplateRepository) *RenderNotificationTemplate {
	return &RenderNotificationTemplate{repo: repo}
}

func (uc *RenderNotificationTemplate) Execute(ctx context.Context, id string, req domain.RenderRequest) (*domain.RenderResponse, error) {
	t, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	resp := t.Render(req)
	return &resp, nil
}

var _ portin.RenderNotificationTemplateUseCase = (*RenderNotificationTemplate)(nil)

// ---- TestSendNotificationTemplate ----

// TestSendNotificationTemplate renders a template and simulates sending a notification.
// In production this would integrate with an email/push/SMS provider.
// For parity with the Java backend it renders the template and returns the rendered output.
type TestSendNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewTestSendNotificationTemplate creates a TestSendNotificationTemplate use case.
func NewTestSendNotificationTemplate(repo portout.NotificationTemplateRepository) *TestSendNotificationTemplate {
	return &TestSendNotificationTemplate{repo: repo}
}

func (uc *TestSendNotificationTemplate) Execute(ctx context.Context, in portin.TestSendInput) (*domain.RenderResponse, error) {
	t, err := uc.repo.FindByID(ctx, in.TemplateID)
	if err != nil {
		return nil, err
	}
	if !t.IsAtivo() {
		return nil, apierr.Unprocessable("template inativo não pode ser enviado")
	}
	if in.Destinatario == "" {
		return nil, apierr.BadRequest("destinatário é obrigatório")
	}
	resp := t.Render(domain.RenderRequest{Variaveis: in.Variaveis})
	return &resp, nil
}

var _ portin.TestSendNotificationTemplateUseCase = (*TestSendNotificationTemplate)(nil)

// ---- GetByTypeNotificationTemplate ----

// GetByTypeNotificationTemplate retrieves the template that matches a given TemplateType.
type GetByTypeNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewGetByTypeNotificationTemplate creates a GetByTypeNotificationTemplate use case.
func NewGetByTypeNotificationTemplate(repo portout.NotificationTemplateRepository) *GetByTypeNotificationTemplate {
	return &GetByTypeNotificationTemplate{repo: repo}
}

func (uc *GetByTypeNotificationTemplate) Execute(ctx context.Context, tipo domain.TemplateType) (*domain.NotificationTemplate, error) {
	return uc.repo.FindByType(ctx, tipo)
}

var _ portin.GetByTypeNotificationTemplateUseCase = (*GetByTypeNotificationTemplate)(nil)

// ---- ListByCategoriaNotificationTemplate ----

// ListByCategoriaNotificationTemplate lists templates filtered by TemplateCategoria.
type ListByCategoriaNotificationTemplate struct {
	repo portout.NotificationTemplateRepository
}

// NewListByCategoriaNotificationTemplate creates a ListByCategoriaNotificationTemplate use case.
func NewListByCategoriaNotificationTemplate(repo portout.NotificationTemplateRepository) *ListByCategoriaNotificationTemplate {
	return &ListByCategoriaNotificationTemplate{repo: repo}
}

func (uc *ListByCategoriaNotificationTemplate) Execute(ctx context.Context, categoria domain.TemplateCategoria, page, pageSize int) ([]domain.NotificationTemplate, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return uc.repo.FindByCategoria(ctx, categoria, page, pageSize)
}

var _ portin.ListByCategoriaNotificationTemplateUseCase = (*ListByCategoriaNotificationTemplate)(nil)
