package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RemovePlaylist implements portin.RemovePlaylistUseCase.
type RemovePlaylist struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewRemovePlaylist creates a new RemovePlaylist use case.
func NewRemovePlaylist(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *RemovePlaylist {
	return &RemovePlaylist{repo: repo, auth: auth}
}

// Execute removes a playlist link from an event (organizador only).
// Idempotent: no error when the playlist does not exist.
func (uc *RemovePlaylist) Execute(ctx context.Context, in portin.RemovePlaylistInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("remove playlist: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: remove (idempotent — $pull by id)
	if err := uc.repo.RemovePlaylist(ctx, in.EventoID, in.PlaylistID); err != nil {
		return fmt.Errorf("remove playlist: %w", err)
	}
	return nil
}
