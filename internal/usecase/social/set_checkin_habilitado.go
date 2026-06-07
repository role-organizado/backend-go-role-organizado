package social

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/social"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// SetCheckinHabilitado implements portin.SetCheckinHabilitadoUseCase.
type SetCheckinHabilitado struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewSetCheckinHabilitado creates a new SetCheckinHabilitado use case.
func NewSetCheckinHabilitado(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *SetCheckinHabilitado {
	return &SetCheckinHabilitado{repo: repo, auth: auth}
}

// Execute enables or disables check-in for an event (organizador only).
// Uses upsert to set the checkinHabilitado flag.
func (uc *SetCheckinHabilitado) Execute(ctx context.Context, in portin.SetCheckinHabilitadoInput) error {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return err
	}
	// Step 3: auth — organizador only
	ok, err := uc.auth.IsOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return fmt.Errorf("set checkin habilitado: verificar autorização: %w", err)
	}
	if !ok {
		return apierr.Forbidden("FORBIDDEN")
	}
	// Step 4: set flag (upsert)
	if err := uc.repo.SetCheckinHabilitado(ctx, in.EventoID, in.Habilitado); err != nil {
		return fmt.Errorf("set checkin habilitado: %w", err)
	}
	return nil
}
