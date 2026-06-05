package out

import "context"

// ParticipanteRepository defines persistence operations for event participants.
type ParticipanteRepository interface {
	// SaveOrganizador registers the event creator as an ORGANIZADOR participant.
	SaveOrganizador(ctx context.Context, eventoID, usuarioID string) error
}
