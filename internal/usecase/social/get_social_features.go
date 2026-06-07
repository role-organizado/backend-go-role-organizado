package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// GetSocialFeatures implements portin.GetSocialFeaturesUseCase.
type GetSocialFeatures struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewGetSocialFeatures creates a new GetSocialFeatures use case.
func NewGetSocialFeatures(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *GetSocialFeatures {
	return &GetSocialFeatures{repo: repo, auth: auth}
}

// Execute retrieves the social features document for an event.
// Returns an empty structure (no lazy-create) when no document exists yet.
// Auth: organizador OR confirmed participant.
func (uc *GetSocialFeatures) Execute(ctx context.Context, in portin.GetSocialFeaturesInput) (*domain.EventoSocialFeatures, error) {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — organizador or confirmed participant
	ok, err := uc.auth.IsParticipanteConfirmadoOuOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("get social features: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: read doc (no lazy-create)
	doc, err := uc.repo.FindByEventoID(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("get social features: %w", err)
	}
	if doc == nil {
		doc = &domain.EventoSocialFeatures{EventoID: in.EventoID}
	}
	return doc, nil
}
