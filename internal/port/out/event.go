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

	// FindAllByIDs returns the events whose IDs are in the given list. IDs not
	// found are silently omitted (no error). Used by the BuscarSummaries batch
	// endpoint.
	FindAllByIDs(ctx context.Context, ids []string) ([]event.Evento, error)
	// UpdateFase atomically updates the fase field and atualizado_em.
	UpdateFase(ctx context.Context, id string, fase event.EventoFase) error
	// UpdatePoliticaConvidados atomically updates politica_convidados.
	UpdatePoliticaConvidados(ctx context.Context, id, politica string) error
	// AddImagens appends a list of images to the event's imagens array.
	AddImagens(ctx context.Context, id string, imagens []event.EventoImagem) error
	// UpdateDetalhes patches editable detail fields: nome/tipo/descricao/local/
	// data_inicio/data_fim/endereco. Returns the updated event.
	UpdateDetalhes(ctx context.Context, e *event.Evento) (*event.Evento, error)
}

// EventoDraftRepository defines persistence operations for event drafts.
type EventoDraftRepository interface {
	FindByID(ctx context.Context, id string) (*event.EventoDraft, error)
	FindByUsuarioID(ctx context.Context, usuarioID string) ([]event.EventoDraft, error)
	Save(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	Update(ctx context.Context, d *event.EventoDraft) (*event.EventoDraft, error)
	DeleteByID(ctx context.Context, id string) error
}
