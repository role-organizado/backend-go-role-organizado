package participant

import (
	"context"
	"fmt"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// CancelParticipantInstallments implements portin.CancelParticipantInstallmentsUseCase
// from the participant-lifecycle perspective: it is invoked by the
// ParticipantLifecycleWorkflow when an organizer confirms removing a participant,
// cancelling that participant's still-open (PENDING/OVERDUE) installments.
//
// Authorization mirrors the payment-side use case: only the event organizer may
// cancel a participant's installments. Status filtering is enforced at the
// repository layer via CancelByParticipant.
type CancelParticipantInstallments struct {
	installments portout.PaymentInstallmentRepository
	eventos      portout.EventoRepository
}

// NewCancelParticipantInstallments creates a new CancelParticipantInstallments use case.
func NewCancelParticipantInstallments(
	installments portout.PaymentInstallmentRepository,
	eventos portout.EventoRepository,
) *CancelParticipantInstallments {
	return &CancelParticipantInstallments{installments: installments, eventos: eventos}
}

// Execute cancels all PENDING and OVERDUE installments for the target participant
// in the given event, returning the number of records updated.
func (uc *CancelParticipantInstallments) Execute(ctx context.Context, in portin.CancelParticipantInstallmentsInput) (int64, error) {
	ev, err := uc.eventos.FindByID(ctx, in.EventID)
	if err != nil {
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

var _ portin.CancelParticipantInstallmentsUseCase = (*CancelParticipantInstallments)(nil)
