package listapresentes

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/listapresentes"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- AddItem ----

// AddItem implements portin.AddItemUseCase.
type AddItem struct {
	repo       portout.ListaPresentesRepository
	eventoRepo portout.EventoRepository
}

// NewAddItem creates a new AddItem use case.
func NewAddItem(repo portout.ListaPresentesRepository, eventoRepo portout.EventoRepository) *AddItem {
	return &AddItem{repo: repo, eventoRepo: eventoRepo}
}

// Execute validates event ownership and creates a new gift list item.
func (uc *AddItem) Execute(ctx context.Context, in portin.AddItemInput) (*domain.ListaPresentesItem, error) {
	if in.EventoID == "" {
		return nil, apierr.BadRequest("eventoId é obrigatório")
	}
	if in.OwnerUserID == "" {
		return nil, apierr.BadRequest("ownerUserId é obrigatório")
	}
	if in.Nome == "" {
		return nil, apierr.BadRequest("nome é obrigatório")
	}
	if in.Quantidade <= 0 {
		return nil, apierr.BadRequest("quantidade deve ser maior que zero")
	}

	// Validate that the event exists and the caller is the owner.
	evento, err := uc.eventoRepo.FindByID(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("buscar evento: %w", err)
	}
	if evento.UsuarioID != in.OwnerUserID {
		return nil, apierr.Forbidden("apenas o organizador do evento pode adicionar itens à lista")
	}

	now := time.Now()
	item := &domain.ListaPresentesItem{
		ID:          uuid.New().String(),
		EventoID:    in.EventoID,
		OwnerUserID: in.OwnerUserID,
		Nome:        in.Nome,
		Descricao:   in.Descricao,
		URLProduto:  in.URLProduto,
		Valor:       in.Valor,
		Quantidade:  in.Quantidade,
		Reservado:   0,
		Status:      domain.StatusDisponivel,
		CriadoEm:   now,
		UpdatedAt:   now,
	}
	return uc.repo.Save(ctx, item)
}

// ---- GetItem ----

// GetItem implements portin.GetItemUseCase.
type GetItem struct {
	repo portout.ListaPresentesRepository
}

// NewGetItem creates a new GetItem use case.
func NewGetItem(repo portout.ListaPresentesRepository) *GetItem {
	return &GetItem{repo: repo}
}

// Execute retrieves a gift list item by ID.
func (uc *GetItem) Execute(ctx context.Context, id string) (*domain.ListaPresentesItem, error) {
	return uc.repo.FindByID(ctx, id)
}

// ---- ListItems ----

// ListItems implements portin.ListItemsUseCase.
type ListItems struct {
	repo portout.ListaPresentesRepository
}

// NewListItems creates a new ListItems use case.
func NewListItems(repo portout.ListaPresentesRepository) *ListItems {
	return &ListItems{repo: repo}
}

// Execute lists all items for the given event.
func (uc *ListItems) Execute(ctx context.Context, eventoID string) ([]*domain.ListaPresentesItem, error) {
	if eventoID == "" {
		return nil, apierr.BadRequest("eventoId é obrigatório")
	}
	return uc.repo.FindByEventoID(ctx, eventoID)
}

// ---- ReservarItem ----

// ReservarItem implements portin.ReservarItemUseCase.
type ReservarItem struct {
	repo portout.ListaPresentesRepository
}

// NewReservarItem creates a new ReservarItem use case.
func NewReservarItem(repo portout.ListaPresentesRepository) *ReservarItem {
	return &ReservarItem{repo: repo}
}

// Execute reserves a gift list item for a guest.
// Returns an error if the item has no available quantity.
func (uc *ReservarItem) Execute(ctx context.Context, in portin.ReservarItemInput) (*domain.ListaPresentesItem, error) {
	if in.ItemID == "" {
		return nil, apierr.BadRequest("itemId é obrigatório")
	}

	item, err := uc.repo.FindByID(ctx, in.ItemID)
	if err != nil {
		return nil, err
	}
	if !item.CanReserve() {
		return nil, apierr.Unprocessable(fmt.Sprintf("item sem quantidade disponível (reservado: %d, quantidade: %d)", item.Reservado, item.Quantidade))
	}

	novoReservado := item.Reservado + 1
	novoStatus := domain.StatusDisponivel
	if novoReservado >= item.Quantidade {
		novoStatus = domain.StatusReservado
	}

	return uc.repo.UpdateStatus(ctx, in.ItemID, novoStatus, novoReservado, in.GuestID)
}

// ---- RemoveItem ----

// RemoveItem implements portin.RemoveItemUseCase.
type RemoveItem struct {
	repo portout.ListaPresentesRepository
}

// NewRemoveItem creates a new RemoveItem use case.
func NewRemoveItem(repo portout.ListaPresentesRepository) *RemoveItem {
	return &RemoveItem{repo: repo}
}

// Execute removes a gift list item, only if the caller is the owner.
func (uc *RemoveItem) Execute(ctx context.Context, itemID, userID string) error {
	item, err := uc.repo.FindByID(ctx, itemID)
	if err != nil {
		return err
	}
	if !item.IsOwner(userID) {
		return apierr.Forbidden("apenas o organizador pode remover itens da lista")
	}
	return uc.repo.Delete(ctx, itemID)
}
