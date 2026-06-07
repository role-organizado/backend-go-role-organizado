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

// DoCheckin implements portin.DoCheckinUseCase.
type DoCheckin struct {
	repo portout.SocialFeaturesRepository
	auth portout.EventoAuthPort
}

// NewDoCheckin creates a new DoCheckin use case.
func NewDoCheckin(repo portout.SocialFeaturesRepository, auth portout.EventoAuthPort) *DoCheckin {
	return &DoCheckin{repo: repo, auth: auth}
}

// Execute registers a participant's check-in to an event.
// Returns 409 CHECKIN_NOT_ENABLED when checkin is disabled.
// Returns 409 CHECKIN_ALREADY_REGISTERED when the user has already checked in.
func (uc *DoCheckin) Execute(ctx context.Context, in portin.DoCheckinInput) (*domain.Checkin, error) {
	// Steps 1+2: event exists + fase >= AGUARDANDO_ACEITE
	if err := uc.auth.FaseAtLeast(ctx, in.EventoID, domain.FaseAguardandoAceite); err != nil {
		return nil, err
	}
	// Step 3: auth — confirmed participant (or organizer)
	ok, err := uc.auth.IsParticipanteConfirmadoOuOrganizador(ctx, in.EventoID, in.UsuarioID)
	if err != nil {
		return nil, fmt.Errorf("do checkin: verificar autorização: %w", err)
	}
	if !ok {
		return nil, apierr.Forbidden("FORBIDDEN")
	}
	// Step 4a: read doc to validate checkinHabilitado flag
	doc, err := uc.repo.FindByEventoID(ctx, in.EventoID)
	if err != nil {
		return nil, fmt.Errorf("do checkin: ler features: %w", err)
	}
	if doc == nil || !doc.CheckinHabilitado {
		return nil, apierr.Conflict("CHECKIN_NOT_ENABLED")
	}
	// Step 4b: register checkin (atomic — repo returns Conflict if already registered)
	checkin := domain.Checkin{
		UsuarioID: in.UsuarioID,
		Nome:      in.UsuarioNome,
		Timestamp: time.Now(),
	}
	if err := uc.repo.AddCheckin(ctx, in.EventoID, checkin); err != nil {
		return nil, fmt.Errorf("do checkin: %w", err)
	}
	return &checkin, nil
}
