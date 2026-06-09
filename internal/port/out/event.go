package out

import (
	"context"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
)

// EventoQueryFiltros holds optional filter parameters for event queries.
type EventoQueryFiltros struct {
	Status        *string
	Tipo          *string
	DataInicioGte *time.Time
	DataInicioLte *time.Time
	// Cursor is a base64-encoded offset for cursor-based pagination.
	Cursor *string
	Limit  int
}

// EventosCursorPage is the result of a cursor-paginated event query.
type EventosCursorPage struct {
	Eventos     []event.Evento
	Total       int64
	NextCursor  *string
	HasNextPage bool
	Limit       int
}

// EventoRepository defines persistence operations for events.
type EventoRepository interface {
	FindByID(ctx context.Context, id string) (*event.Evento, error)
	FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]event.Evento, int64, error)
	FindByUsuarioIDCursor(ctx context.Context, usuarioID string, filtros EventoQueryFiltros) (EventosCursorPage, error)
	FindAll(ctx context.Context, page, pageSize int) ([]event.Evento, int64, error)
	Save(ctx context.Context, e *event.Evento) (*event.Evento, error)
	Update(ctx context.Context, e *event.Evento) (*event.Evento, error)
	DeleteByID(ctx context.Context, id string) error
	AddConvidados(ctx context.Context, eventoID string, convidados []event.Convidado) error
}

// EventoDraftRepository defines persistence operations for event drafts.
type EventoDraftRepository interface {
	FindByID(ctx context.Context, id string) (*event.EventoDraft, error)
	FindByUsuarioID(ctx context.Context, usuarioID string) ([]event.EventoDraft, error)
	Save(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	Update(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	DeleteByID(ctx context.Context, id string) error
}
