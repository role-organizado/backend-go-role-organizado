package out

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/listapresentes"
)

// ListaPresentesRepository defines persistence operations for gift list items.
type ListaPresentesRepository interface {
	Save(ctx context.Context, item *listapresentes.ListaPresentesItem) (*listapresentes.ListaPresentesItem, error)
	FindByID(ctx context.Context, id string) (*listapresentes.ListaPresentesItem, error)
	FindByEventoID(ctx context.Context, eventoID string) ([]*listapresentes.ListaPresentesItem, error)
	UpdateStatus(ctx context.Context, id string, status listapresentes.ListaItemStatus, reservado int, guestID string) (*listapresentes.ListaPresentesItem, error)
	Delete(ctx context.Context, id string) error
}
