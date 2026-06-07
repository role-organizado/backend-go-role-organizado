package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UpdateBringListItem implements portin.UpdateBringListItemUseCase.
type UpdateBringListItem struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewUpdateBringListItem creates a new UpdateBringListItem use case.
func NewUpdateBringListItem(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *UpdateBringListItem {
	return &UpdateBringListItem{repo: repo, auth: auth}
}

// Execute updates nome and/or quantidade of a bring list item (organizador only).
// Uses the positional $ operator at the adapter level for atomic update.
func (uc *UpdateBringListItem) Execute(ctx context.Context, in portin.UpdateBringListItemInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("update bring list item: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: update item
	if err := uc.repo.UpdateBringListItem(ctx, in.EventoID, in.ItemID, in.Nome, in.Quantidade); err != nil {
		return fmt.Errorf("update bring list item: %w", err)
	}
	return nil
}
