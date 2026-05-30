package in

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// CreatePagamentoInput contains fields to create a new recurring payment.
type CreatePagamentoInput struct {
	EventoID        string
	UsuarioID       string
	Descricao       string
	Valor           float64
	MetodoPagamento domain.MetodoPagamento
	DataVencimento  time.Time
	Observacao      string
}

// UpdatePagamentoInput holds optional fields to update a payment.
type UpdatePagamentoInput struct {
	Descricao       *string
	Valor           *float64
	DataVencimento  *time.Time
	Observacao      *string
}

// ConfirmarPagamentoInput holds data to confirm a payment as settled.
type ConfirmarPagamentoInput struct {
	DataPagamento time.Time
	Comprovante   string // optional GridFS file ID
}

// UpsertConfigPagamentoInput holds payment config for an event.
type UpsertConfigPagamentoInput struct {
	EventoID         string
	UsuarioID        string
	MetodosPagamento []domain.MetodoPagamento
	PrazoPagamento   *time.Time
	ChavePix         string
	InstrucoesBoleto string
}

// CreatePagamentoUseCase creates a new payment record.
type CreatePagamentoUseCase interface {
	Execute(ctx context.Context, in CreatePagamentoInput) (*domain.PagamentoMensal, error)
}

// GetPagamentoUseCase retrieves a payment by ID.
type GetPagamentoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) (*domain.PagamentoMensal, error)
}

// ListPagamentosUseCase lists all payments for an event.
type ListPagamentosUseCase interface {
	Execute(ctx context.Context, eventoID, requesterID string) ([]domain.PagamentoMensal, error)
}

// UpdatePagamentoUseCase partially updates a payment.
type UpdatePagamentoUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in UpdatePagamentoInput) (*domain.PagamentoMensal, error)
}

// DeletePagamentoUseCase removes a payment.
type DeletePagamentoUseCase interface {
	Execute(ctx context.Context, id, requesterID string) error
}

// ConfirmarPagamentoUseCase marks a payment as paid.
type ConfirmarPagamentoUseCase interface {
	Execute(ctx context.Context, id, requesterID string, in ConfirmarPagamentoInput) (*domain.PagamentoMensal, error)
}

// UpsertConfigPagamentoUseCase creates or updates payment config for an event.
type UpsertConfigPagamentoUseCase interface {
	Execute(ctx context.Context, in UpsertConfigPagamentoInput) (*domain.EventoConfigPagamento, error)
}

// GetConfigPagamentoUseCase retrieves payment config for an event.
type GetConfigPagamentoUseCase interface {
	Execute(ctx context.Context, eventoID, requesterID string) (*domain.EventoConfigPagamento, error)
}
