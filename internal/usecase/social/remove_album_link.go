package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RemoveAlbumLink implements portin.RemoveAlbumLinkUseCase.
type RemoveAlbumLink struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewRemoveAlbumLink creates a new RemoveAlbumLink use case.
func NewRemoveAlbumLink(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *RemoveAlbumLink {
	return &RemoveAlbumLink{repo: repo, auth: auth}
}

// Execute removes a photo album link from an event (organizador ONLY).
// Idempotent: no error when the link does not exist.
//
// NOTE: auth is stricter than AddAlbumLink — ONLY the organizador can remove.
func (uc *RemoveAlbumLink) Execute(ctx context.Context, in portin.RemoveAlbumLinkInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador ONLY (stricter than add)
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("remove album link: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: remove (idempotent — $pull by id)
	if err := uc.repo.RemoveAlbumLink(ctx, in.EventoID, in.LinkID); err != nil {
		return fmt.Errorf("remove album link: %w", err)
	}
	return nil
}
