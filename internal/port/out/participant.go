package out

import (
	"context"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
)

// ParticipanteRepository defines write-side persistence for event participants.
type ParticipanteRepository interface {
	// SaveOrganizador registers the event creator as an ORGANIZADOR participant.
	SaveOrganizador(ctx context.Context, eventoID, usuarioID string) error
	// FindParticipationIDsByUserID returns the IDs of all participation records
	// for the given user. Used by the BUG5/spec-096 dual-search fix in installment
	// queries so that installments stored under a participation UUID are returned.
	FindParticipationIDsByUserID(ctx context.Context, userID string) ([]string, error)
	// IsParticipantOfEvent reports whether the user has any participation record
	// in the given event (any papel or status).
	IsParticipantOfEvent(ctx context.Context, eventID, userID string) (bool, error)
}

// ParticipantRepository defines read-side persistence for event participants (finance domain).
type ParticipantRepository interface {
	// FindByUserID returns all participations for the given user.
	FindByUserID(ctx context.Context, userID string) ([]domain.Participant, error)
	// FindByEventID returns a page of participants for the given event.
	FindByEventID(ctx context.Context, eventID string, page, size int) ([]domain.Participant, int64, error)
	// FindByEventIDAndUserID returns the participation for a specific user in a specific event.
	// Returns an error (e.g. apierr.NotFound) if the participation does not exist.
	FindByEventIDAndUserID(ctx context.Context, eventID, userID string) (*domain.Participant, error)
}

// PaymentInstallmentRepository defines persistence for payment installments.
type FinanceInstallmentRepository interface {
	// FindByEventID returns all installments for the given event.
	FindByEventID(ctx context.Context, eventID string) ([]domain.PaymentInstallment, error)
	// FindByParticipantID returns all installments for a specific participant in an event.
	FindByParticipantID(ctx context.Context, eventID, participantID string) ([]domain.PaymentInstallment, error)
	// FindPendingByEventID returns all PENDING or OVERDUE installments for the given event.
	FindPendingByEventID(ctx context.Context, eventID string) ([]domain.PaymentInstallment, error)
}
