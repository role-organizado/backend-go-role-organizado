package out

import "context"

// ParticipanteRepository defines persistence operations for event participants.
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
