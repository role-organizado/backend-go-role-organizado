package in

import (
	"context"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
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

// ============================================================================
// Notification Stages — input ports
// ============================================================================

// UpsertNotificationStageInput is the request body for creating/replacing a stage.
type UpsertNotificationStageInput struct {
	Locale        string
	EventType     string
	SuccessPolicy notification.NotificationStageSuccessPolicy
	Levels        []NotificationStageLevelInput
}

// NotificationStageLevelInput is one escalation level of an upsert request.
type NotificationStageLevelInput struct {
	Order     int
	Templates []NotificationStageTemplateInput
}

// NotificationStageTemplateInput is one channel template within a level.
type NotificationStageTemplateInput struct {
	Canal     notification.NotificationChannel
	Nome      string
	Assunto   string
	Corpo     string
	Variaveis []string
	Metadados map[string]any
	Ativo     *bool
}

// TestSendStagesInput drives the multi-stage test-send orchestrator.
type TestSendStagesInput struct {
	StageKey     string
	EventType    string
	Destinatario string
	Variaveis    map[string]string
}

// StageChannelResult is the per-channel outcome of a test send within a level.
type StageChannelResult struct {
	Canal   notification.NotificationChannel `json:"canal"`
	Assunto string                           `json:"assunto,omitempty"`
	Corpo   string                           `json:"corpo"`
	Enviado bool                             `json:"enviado"`
}

// StageLevelResult is the outcome of one escalation level.
type StageLevelResult struct {
	Order    int                  `json:"order"`
	Canais   []StageChannelResult `json:"canais"`
	Sucesso  bool                 `json:"sucesso"`
	Parou    bool                 `json:"parou"` // true when escalation stopped at this level
}

// TestSendStagesResult is the aggregated outcome of the orchestrator.
type TestSendStagesResult struct {
	Key             string                                      `json:"key"`
	ResolvedEvent   string                                      `json:"resolvedEventType"`
	SuccessPolicy   notification.NotificationStageSuccessPolicy `json:"successPolicy"`
	Destinatario    string                                      `json:"destinatario"`
	Niveis          []StageLevelResult                          `json:"niveis"`
	Sucesso         bool                                        `json:"sucesso"`
	TotalEnviados   int                                         `json:"totalEnviados"`
}

// ListNotificationStagesUseCase lists stages, optionally filtered by eventType.
type ListNotificationStagesUseCase interface {
	Execute(ctx context.Context, eventType string) ([]notification.NotificationStageConfig, error)
}

// GetNotificationStageUseCase fetches a single stage by key (+ optional eventType).
type GetNotificationStageUseCase interface {
	Execute(ctx context.Context, stageKey, eventType string) (*notification.NotificationStageConfig, error)
}

// UpsertNotificationStageUseCase replaces a stage's templates atomically.
type UpsertNotificationStageUseCase interface {
	Execute(ctx context.Context, stageKey string, in UpsertNotificationStageInput) (*notification.NotificationStageConfig, error)
}

// DeleteNotificationStageUseCase removes a stage (all its template rows).
type DeleteNotificationStageUseCase interface {
	Execute(ctx context.Context, stageKey, eventType string) error
}

// TestSendStagesUseCase renders and simulates the multi-level escalation send.
type TestSendStagesUseCase interface {
	Execute(ctx context.Context, in TestSendStagesInput) (*TestSendStagesResult, error)
}
