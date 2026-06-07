package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RemoveBringListItem implements portin.RemoveBringListItemUseCase.
type RemoveBringListItem struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewRemoveBringListItem creates a new RemoveBringListItem use case.
func NewRemoveBringListItem(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *RemoveBringListItem {
	return &RemoveBringListItem{repo: repo, auth: auth}
}

// Execute removes an item from the event bring list (organizador only).
// Idempotent: no error when the item does not exist ($pull by id).
func (uc *RemoveBringListItem) Execute(ctx context.Context, in portin.RemoveBringListItemInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("remove bring list item: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: remove (idempotent — $pull by id)
	if err := uc.repo.RemoveBringListItem(ctx, in.EventoID, in.ItemID); err != nil {
		return fmt.Errorf("remove bring list item: %w", err)
	}
	return nil
}
