package social

import (
	"context"
	"fmt"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ClaimBringListItem implements portin.ClaimBringListItemUseCase.
type ClaimBringListItem struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewClaimBringListItem creates a new ClaimBringListItem use case.
func NewClaimBringListItem(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *ClaimBringListItem {
	return &ClaimBringListItem{repo: repo, auth: auth}
}

// Execute atomically claims a bring list item for a confirmed participant.
// Returns 409 BRING_LIST_ITEM_ALREADY_CLAIMED when the item is already claimed.
func (uc *ClaimBringListItem) Execute(ctx context.Context, in portin.ClaimBringListItemInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — confirmed participant (or organizer)
	ok, err := uc.auth.IsParticipanteConfirmadoOuOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("claim bring list item: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: atomic claim — repo returns Conflict if already claimed
	if err := uc.repo.ClaimBringListItem(ctx, in.EventoID, in.ItemID, in.UsuarioID, in.UsuarioNome, time.Now()); err != nil {
		return fmt.Errorf("claim bring list item: %w", err)
	}
	return nil
}
