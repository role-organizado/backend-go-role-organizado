package payment

import (
	"context"
	"fmt"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// GetInstallment implements portin.GetInstallmentUseCase.
//
// Returns 404 when the installment does not exist and 403 when the requester
// is not a participant of the event the installment belongs to.
type GetInstallment struct {
	installments  portout.PaymentInstallmentRepository
	participantes portout.ParticipanteRepository
}

// NewGetInstallment creates a new GetInstallment use case.
func NewGetInstallment(
	installments portout.PaymentInstallmentRepository,
	participantes portout.ParticipanteRepository,
) *GetInstallment {
	return &GetInstallment{
		installments:  installments,
		participantes: participantes,
	}
}

// Execute retrieves a single installment, enforcing access control.
func (uc *GetInstallment) Execute(ctx context.Context, installmentID, requesterID string) (*domain.PaymentInstallment, error) {
	inst, err := uc.installments.FindByID(ctx, installmentID)
	if err != nil {
		// Propagates apierr.NotFound (404) or Internal (500) from the repo.
		return nil, err
	}

	// Authorization: requester must be a participant of the associated event.
	isParticipant, err := uc.participantes.IsParticipantOfEvent(ctx, inst.EventID, requesterID)
	if err != nil {
		return nil, fmt.Errorf("get installment: check participant: %w", err)
	}
	if !isParticipant {
		return nil, apierr.Forbidden("acesso negado: você não é participante deste evento")
	}

	return inst, nil
}
