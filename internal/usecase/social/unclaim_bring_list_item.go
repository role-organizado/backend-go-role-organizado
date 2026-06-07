package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// UnclaimBringListItem implements portin.UnclaimBringListItemUseCase.
type UnclaimBringListItem struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewUnclaimBringListItem creates a new UnclaimBringListItem use case.
func NewUnclaimBringListItem(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *UnclaimBringListItem {
	return &UnclaimBringListItem{repo: repo, auth: auth}
}

// Execute releases the claim on a bring list item.
// Idempotent: clears claimedBy, claimedByNome, and claimedAt.
// Auth: confirmed participant OR organizador.
func (uc *UnclaimBringListItem) Execute(ctx context.Context, in portin.UnclaimBringListItemInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — confirmed participant or organizador
	ok, err := uc.auth.IsParticipanteConfirmadoOuOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("unclaim bring list item: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: clear claim fields (idempotent)
	if err := uc.repo.UnclaimBringListItem(ctx, in.EventoID, in.ItemID); err != nil {
		return fmt.Errorf("unclaim bring list item: %w", err)
	}
	return nil
}
