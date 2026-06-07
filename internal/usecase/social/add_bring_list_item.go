package social

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// AddBringListItem implements portin.AddBringListItemUseCase.
type AddBringListItem struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewAddBringListItem creates a new AddBringListItem use case.
func NewAddBringListItem(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *AddBringListItem {
	return &AddBringListItem{repo: repo, auth: auth}
}

// Execute adds an item to the event bring list (organizador only).
// Validates that nome is provided. Generates a UUID for the item ID.
func (uc *AddBringListItem) Execute(ctx context.Context, in portin.AddBringListItemInput) (*domain.BringListItem, error) {
	// Input validation (400 BadRequest — fast-fail before DB)
	if in.Nome == "" {
		return nil, apierr.BadRequest("nome é obrigatório")
	}
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("add bring list item: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: lazy-create + add item
	if _, err := uc.repo.FindOrCreate(ctx, in.EventoID); err != nil {
		return nil, fmt.Errorf("add bring list item: find or create: %w", err)
	}
	item := domain.BringListItem{
		ID:         uuid.New().String(),
		Nome:       in.Nome,
		Quantidade: in.Quantidade,
	}
	if err := uc.repo.AddBringListItem(ctx, in.EventoID, item); err != nil {
		return nil, fmt.Errorf("add bring list item: %w", err)
	}
	return &item, nil
}
