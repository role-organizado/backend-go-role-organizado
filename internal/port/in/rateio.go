package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
)

// ---- Input types ----

// CreateRateioInput holds data for creating a new rateio.
type CreateRateioInput struct {
	EventoID            string
	UsuarioID           string
	Tipo                rateio.TipoRateio
	Descricao           string
	ValorTotal          float64
	NumeroParticipantes int
	Itens               []CreateRateioItemInput
}

// CreateRateioItemInput holds data for a rateio line-item.
type CreateRateioItemInput struct {
	Descricao  string
	Valor      float64
	Quantidade int
}

// UpdateRateioInput holds partial updates for a rateio.
type UpdateRateioInput struct {
	Descricao           *string
	ValorTotal          *float64
	NumeroParticipantes *int
	Itens               []CreateRateioItemInput
}

// FecharRateioInput holds participants for closing a rateio.
type FecharRateioInput struct {
	Participantes []FecharParticipanteInput
}

// FecharParticipanteInput defines a participant's share at closing.
type FecharParticipanteInput struct {
	UsuarioID  string
	Valor      float64
	Percentual float64
}

// ---- Use case interfaces ----

// CreateRateioUseCase creates a new rateio for an event.
type CreateRateioUseCase interface {
	Execute(ctx context.Context, in CreateRateioInput) (*rateio.Rateio, error)
}

// GetRateioUseCase retrieves a rateio by ID.
type GetRateioUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*rateio.Rateio, error)
}

// ListRateiosUseCase lists rateios for an event.
type ListRateiosUseCase interface {
	Execute(ctx context.Context, eventoID, requesterID string) ([]rateio.Rateio, error)
}

// UpdateRateioUseCase applies partial updates to an open rateio.
type UpdateRateioUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpdateRateioInput) (*rateio.Rateio, error)
}

// DeleteRateioUseCase removes a rateio.
type DeleteRateioUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// PreviewRateioUseCase calculates a cost split preview without persisting.
type PreviewRateioUseCase interface {
	Execute(ctx context.Context, id, requesterID string, participantes []string) (*rateio.PreviewResult, error)
}

// FecharRateioUseCase closes a rateio, creating a versioned snapshot.
type FecharRateioUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in FecharRateioInput) (*rateio.RateioFechamento, error)
}

// GetFechamentosUseCase lists all closings for a rateio.
type GetFechamentosUseCase interface {
	Execute(ctx context.Context, rateioID, requesterID string) ([]rateio.RateioFechamento, error)
}
