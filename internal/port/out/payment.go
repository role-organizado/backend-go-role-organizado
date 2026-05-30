package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
)

// PagamentoMensalRepository is the persistence contract for recurring payments.
type PagamentoMensalRepository interface {
	FindByID(ctx context.Context, id string) (*domain.PagamentoMensal, error)
	FindByEventoID(ctx context.Context, eventoID string) ([]domain.PagamentoMensal, error)
	FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]domain.PagamentoMensal, int64, error)
	FindByEventoIDAndStatus(ctx context.Context, eventoID string, status domain.StatusPagamento) ([]domain.PagamentoMensal, error)
	Save(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error)
	Update(ctx context.Context, p *domain.PagamentoMensal) (*domain.PagamentoMensal, error)
	DeleteByID(ctx context.Context, id string) error
}

// EventoConfigPagamentoRepository is the persistence contract for event payment config.
type EventoConfigPagamentoRepository interface {
	FindByEventoID(ctx context.Context, eventoID string) (*domain.EventoConfigPagamento, error)
	Save(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error)
	Update(ctx context.Context, cfg *domain.EventoConfigPagamento) (*domain.EventoConfigPagamento, error)
}
