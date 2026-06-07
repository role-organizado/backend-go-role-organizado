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

// AddPlaylist implements portin.AddPlaylistUseCase.
type AddPlaylist struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewAddPlaylist creates a new AddPlaylist use case.
func NewAddPlaylist(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *AddPlaylist {
	return &AddPlaylist{repo: repo, auth: auth}
}

// Execute adds a playlist link to an event (organizador only, max 3).
// Provider and embedURL are auto-detected from the URL.
// Returns 400 MAX_PLAYLISTS_REACHED when the limit is exceeded.
func (uc *AddPlaylist) Execute(ctx context.Context, in portin.AddPlaylistInput) (*domain.PlaylistLink, error) {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("add playlist: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: check limit then add
	doc, err := uc.repo.FindOrCreate(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("add playlist: find or create: %w", err)
	}
	if len(doc.Playlists) >= domain.MaxPlaylists {
		return nil, apierr.BadRequest("MAX_PLAYLISTS_REACHED")
	}
	provider, embedURL := domain.DetectPlaylistProvider(in.URL)
	playlist := domain.PlaylistLink{
		ID:       uuid.New().String(),
		URL:      in.URL,
		Nome:     in.Nome,
		Provider: provider,
		EmbedURL: embedURL,
	}
	if err := uc.repo.AddPlaylist(ctx, in.EventoID, playlist); err != nil {
		return nil, fmt.Errorf("add playlist: %w", err)
	}
	return &playlist, nil
}
