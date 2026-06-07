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

// AddAlbumLink implements portin.AddAlbumLinkUseCase.
type AddAlbumLink struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewAddAlbumLink creates a new AddAlbumLink use case.
func NewAddAlbumLink(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *AddAlbumLink {
	return &AddAlbumLink{repo: repo, auth: auth}
}

// Execute adds a photo album link to an event (confirmed participant OR organizador, max 5).
// Provider is auto-detected from the URL. adicionadoPor is always set from the JWT (UsuarioID).
// Returns 400 MAX_ALBUM_LINKS_REACHED when the limit is exceeded.
//
// NOTE: auth is more permissive than RemoveAlbumLink — both participants and organizers can add.
func (uc *AddAlbumLink) Execute(ctx context.Context, in portin.AddAlbumLinkInput) (*domain.AlbumLink, error) {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — confirmed participant OR organizador
	ok, err := uc.auth.IsParticipanteConfirmadoOuOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("add album link: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: check limit then add
	doc, err := uc.repo.FindOrCreate(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("add album link: find or create: %w", err)
	}
	if len(doc.AlbumLinks) >= domain.MaxAlbumLinks {
		return nil, apierr.BadRequest("MAX_ALBUM_LINKS_REACHED")
	}
	link := domain.AlbumLink{
		ID:            uuid.New().String(),
		URL:           in.URL,
		Nome:          in.Nome,
		Provider:      domain.DetectAlbumProvider(in.URL),
		AdicionadoPor: in.UsuarioID, // always from JWT, never from body
	}
	if err := uc.repo.AddAlbumLink(ctx, in.EventoID, link); err != nil {
		return nil, fmt.Errorf("add album link: %w", err)
	}
	return &link, nil
}
