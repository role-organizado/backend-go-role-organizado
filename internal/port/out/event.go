package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
)

// EventoRepository defines persistence operations for events.
type EventoRepository interface {
	FindByID(ctx context.Context, id string) (*event.Evento, error)
	FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]event.Evento, int64, error)
	FindAll(ctx context.Context, page, pageSize int) ([]event.Evento, int64, error)
	Save(ctx context.Context, e *event.Evento) (*event.Evento, error)
	Update(ctx context.Context, e *event.Evento) (*event.Evento, error)
	DeleteByID(ctx context.Context, id string) error
}

// EventoDraftRepository defines persistence operations for event drafts.
type EventoDraftRepository interface {
	FindByID(ctx context.Context, id string) (*event.EventoDraft, error)
	FindByUsuarioID(ctx context.Context, usuarioID string) ([]event.EventoDraft, error)
	Save(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	Update(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	DeleteByID(ctx context.Context, id string) error
}
