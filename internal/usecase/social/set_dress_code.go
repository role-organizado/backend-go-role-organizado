package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// SetDressCode implements portin.SetDressCodeUseCase.
type SetDressCode struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewSetDressCode creates a new SetDressCode use case.
func NewSetDressCode(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *SetDressCode {
	return &SetDressCode{repo: repo, auth: auth}
}

// Execute sets or updates the dress code for an event (organizador only).
// Validates that tipo is provided and, when tipo == TEMATICO, that descricaoTematico is also set.
// Lazily creates the social features document if needed.
func (uc *SetDressCode) Execute(ctx context.Context, in portin.SetDressCodeInput) (*domain.EventoSocialFeatures, error) {
	// Input validation (400 BadRequest — fast-fail before DB)
	if in.Tipo == "" {
		return nil, apierr.BadRequest("tipo é obrigatório")
	}
	if domain.DressCodeTipo(in.Tipo) == domain.DressCodeTematico && in.DescricaoTematico == "" {
		return nil, apierr.BadRequest("descricaoTematico é obrigatório quando tipo é TEMATICO")
	}
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("set dress code: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: lazy-create + set dress code
	doc, err := uc.repo.FindOrCreate(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("set dress code: find or create: %w", err)
	}
	dc := &domain.DressCode{
		Tipo:              domain.DressCodeTipo(in.Tipo),
		DescricaoTematico: in.DescricaoTematico,
	}
	if err := uc.repo.SetDressCode(ctx, in.EventoID, dc); err != nil {
		return nil, fmt.Errorf("set dress code: %w", err)
	}
	doc.DressCode = dc
	return doc, nil
}
