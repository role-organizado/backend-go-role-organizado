package payment

import (
	"context"
	"fmt"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// CancelParticipantInstallments implements portin.CancelParticipantInstallmentsUseCase.
//
// Only the event organizer may cancel installments for a participant.
// Status filtering (PENDING / OVERDUE only) is enforced at the repository layer
// via CancelByParticipant.
type CancelParticipantInstallments struct {
	installments  portout.PaymentInstallmentRepository
	participantes portout.ParticipanteRepository
	eventos       portout.EventoRepository
}

// NewCancelParticipantInstallments creates a new CancelParticipantInstallments use case.
func NewCancelParticipantInstallments(
	installments portout.PaymentInstallmentRepository,
	participantes portout.ParticipanteRepository,
	eventos portout.EventoRepository,
) *CancelParticipantInstallments {
	return &CancelParticipantInstallments{
		installments:  installments,
		participantes: participantes,
		eventos:       eventos,
	}
}

// Execute cancels all PENDING and OVERDUE installments for the target participant
// in the given event. Returns the number of records updated.
func (uc *CancelParticipantInstallments) Execute(ctx context.Context, in portin.CancelParticipantInstallmentsInput) (int64, error) {
	// Authorization: only the event organizer may perform this action.
	ev, err := uc.eventos.FindByID(ctx, in.EventID)
	if err != nil {
		// Propagates apierr.NotFound (404) or Internal (500) from the repo.
		return 0, err
	}
	if !ev.IsOwner(in.RequesterID) {
		return 0, apierr.Forbidden("somente o organizador pode cancelar parcelas de participantes")
	}

	count, err := uc.installments.CancelByParticipant(ctx, in.EventID, in.ParticipantID)
	if err != nil {
		return 0, fmt.Errorf("cancel participant installments: %w", err)
	}
	return count, nil
}
