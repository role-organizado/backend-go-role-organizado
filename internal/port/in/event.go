package in

import (
	"context"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
)

// ---- Evento inputs ----

// CreateEventoInput represents the data needed to create a published event.
type CreateEventoInput struct {
	UsuarioID            string
	Nome                 string
	Tipo                 string
	Data                 time.Time
	Descricao            string
	Local                string
	FotoURL              string
	ConvidadosIDs        []string
	PoliticaConvidados   string
	LimiteConvidados     *int
	RateiosHabilitado    bool
	TipoDivisaoRateio    string
	PagamentosHabilitado bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time
	RegrasCustomizadas   string
	PoliticaCancelamento string
}

// UpdateEventoInput holds fields that can be updated on an existing event.
type UpdateEventoInput struct {
	Nome                 string
	Tipo                 string
	Data                 *time.Time
	Descricao            string
	Local                string
	FotoURL              string
	PoliticaConvidados   string
	LimiteConvidados     *int
	RateiosHabilitado    *bool
	TipoDivisaoRateio    string
	PagamentosHabilitado *bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time
	RegrasCustomizadas   string
	PoliticaCancelamento string
}

// CreateEventoUseCase creates a new event.
type CreateEventoUseCase interface {
	Execute(ctx context.Context, in CreateEventoInput) (*event.Evento, error)
}

// GetEventoUseCase retrieves an event by ID.
type GetEventoUseCase interface {
	Execute(ctx context.Context, id string) (*event.Evento, error)
}

// ListEventosUseCase lists events, optionally filtered by owner.
type ListEventosUseCase interface {
	Execute(ctx context.Context, usuarioID *string, page, pageSize int) ([]event.Evento, int64, error)
}

// UpdateEventoUseCase updates an existing event.
type UpdateEventoUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpdateEventoInput) (*event.Evento, error)
}

// DeleteEventoUseCase deletes an event by ID.
type DeleteEventoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// ---- EventoDraft inputs ----

// DraftRateioItem is used in draft upsert inputs.
type DraftRateioItem struct {
	Descricao  string
	Valor      float64
	Quantidade int
}

// UpsertDraftInput represents a draft auto-save payload.
// All fields are optional — only non-nil/non-zero fields are applied.
type UpsertDraftInput struct {
	// Etapa 0
	Nome      *string
	Tipo      *string
	Data      *time.Time
	Descricao *string
	Local     *string

	// Etapa 1
	ConvidadosIDs      []string
	PoliticaConvidados *string
	LimiteConvidados   *int

	// Etapa 2
	RateiosHabilitado *bool
	RateiosItens      []DraftRateioItem
	TipoDivisaoRateio *string

	// Etapa 3
	PagamentosHabilitado *bool
	MetodosPagamento     []string
	PrazoPagamento       *time.Time

	// Etapa 4
	RegrasCustomizadas   *string
	PoliticaCancelamento *string

	// Wizard state
	EtapaAtual      *int
	EtapasCompletas []int

	// Optimistic concurrency: client last-read timestamp
	LastReadAt *time.Time
}

// CreateDraftUseCase creates a new empty draft.
type CreateDraftUseCase interface {
	Execute(ctx context.Context, usuarioID string) (*event.EventoDraft, error)
}

// GetDraftUseCase retrieves a draft by ID.
type GetDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*event.EventoDraft, error)
}

// ListDraftsUseCase lists all drafts for a user.
type ListDraftsUseCase interface {
	Execute(ctx context.Context, usuarioID string) ([]event.EventoDraft, error)
}

// UpdateDraftUseCase applies partial updates to a draft (auto-save).
type UpdateDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpsertDraftInput) (*event.EventoDraft, error)
}

// DeleteDraftUseCase deletes a draft by ID.
type DeleteDraftUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// PublishDraftUseCase converts a completed draft into a published event.
type PublishDraftUseCase interface {
	Execute(ctx context.Context, draftID, requesterID string) (*event.Evento, error)
}
