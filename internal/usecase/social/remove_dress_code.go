package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// RemoveDressCode implements portin.RemoveDressCodeUseCase.
type RemoveDressCode struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewRemoveDressCode creates a new RemoveDressCode use case.
func NewRemoveDressCode(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *RemoveDressCode {
	return &RemoveDressCode{repo: repo, auth: auth}
}

// Execute removes the dress code from an event (organizador only).
// Idempotent: no error when dress code is already absent.
// Calls SetDressCode with nil to unset the field in MongoDB.
func (uc *RemoveDressCode) Execute(ctx context.Context, in portin.RemoveDressCodeInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("remove dress code: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: unset dress code (nil = $unset in the adapter)
	if err := uc.repo.SetDressCode(ctx, in.EventoID, nil); err != nil {
		return fmt.Errorf("remove dress code: %w", err)
	}
	return nil
}
