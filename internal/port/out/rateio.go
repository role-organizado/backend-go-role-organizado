package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/rateio"
)

// RateioRepository defines persistence operations for rateios.
type RateioRepository interface {
	FindByID(ctx context.Context, id string) (*rateio.Rateio, error)
	FindByEventoID(ctx context.Context, eventoID string) ([]rateio.Rateio, error)
	FindByUsuarioID(ctx context.Context, usuarioID string, page, pageSize int) ([]rateio.Rateio, int64, error)
	Save(ctx context.Context, r *rateio.Rateio) (*rateio.Rateio, error)
	Update(ctx context.Context, r *rateio.Rateio) (*rateio.Rateio, error)
	DeleteByID(ctx context.Context, id string) error
}

// RateioItemRepository defines persistence for rateio line-items.
type RateioItemRepository interface {
	FindByRateioID(ctx context.Context, rateioID string) ([]rateio.RateioItem, error)
	Save(ctx context.Context, item *rateio.RateioItem) (*rateio.RateioItem, error)
	Update(ctx context.Context, item *rateio.RateioItem) (*rateio.RateioItem, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByRateioID(ctx context.Context, rateioID string) error
}

// RateioFechamentoRepository defines persistence for rateio closings.
type RateioFechamentoRepository interface {
	FindByRateioID(ctx context.Context, rateioID string) ([]rateio.RateioFechamento, error)
	FindLatestByRateioID(ctx context.Context, rateioID string) (*rateio.RateioFechamento, error)
	Save(ctx context.Context, f *rateio.RateioFechamento) (*rateio.RateioFechamento, error)
}
