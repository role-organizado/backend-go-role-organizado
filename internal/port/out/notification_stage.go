package out

import (
	"context"

	notification "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
)

// NotificationStageRepository persists the individual stage-template rows whose
// keys follow the STAGE__<KEY>__<EVENT>__<CHANNEL>__L<N> convention.
//
// Stages are stored as plain notification templates keyed by Chave; this port is
// intentionally narrow (full scan + save + delete) to mirror the Java
// GerenciarNotificationStagesUseCase which operates over findAll().
type NotificationStageRepository interface {
	// FindAll returns every stage-template row (active and inactive).
	FindAll(ctx context.Context) ([]notification.NotificationStage, error)
	// Save inserts or updates a stage-template row (keyed by Chave).
	Save(ctx context.Context, s *notification.NotificationStage) (*notification.NotificationStage, error)
	// DeleteByID removes a stage-template row by its persistence id.
	DeleteByID(ctx context.Context, id string) error
}
