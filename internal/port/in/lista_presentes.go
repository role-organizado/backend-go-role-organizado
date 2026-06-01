package in

import (
	"context"

	"github.com/role-organizado/backend-go-role-organizado/internal/domain/listapresentes"
)

// AddItemInput holds data needed to add a new item to a gift list.
type AddItemInput struct {
	EventoID    string
	OwnerUserID string
	Nome        string
	Descricao   string
	URLProduto  string
	Valor       int64 // centavos, 0 = qualquer valor
	Quantidade  int
}

// ReservarItemInput holds data needed to reserve a gift list item.
type ReservarItemInput struct {
	ItemID  string
	GuestID string
}

// AddItemUseCase adds a new item to a gift list.
type AddItemUseCase interface {
	Execute(ctx context.Context, in AddItemInput) (*listapresentes.ListaPresentesItem, error)
}

// GetItemUseCase retrieves a single gift list item by ID.
type GetItemUseCase interface {
	Execute(ctx context.Context, id string) (*listapresentes.ListaPresentesItem, error)
}

// ListItemsUseCase lists all items in a gift list for a given event.
type ListItemsUseCase interface {
	Execute(ctx context.Context, eventoID string) ([]*listapresentes.ListaPresentesItem, error)
}

// ReservarItemUseCase reserves a gift list item for a guest.
type ReservarItemUseCase interface {
	Execute(ctx context.Context, in ReservarItemInput) (*listapresentes.ListaPresentesItem, error)
}

// RemoveItemUseCase removes an item from a gift list (owner only).
type RemoveItemUseCase interface {
	Execute(ctx context.Context, itemID, userID string) error
}
