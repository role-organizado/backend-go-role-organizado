package payment

import (
	"context"
	"fmt"

	domain_event "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// excludedEventPhases holds Java lifecycle phases that indicate an event is still
// in a planning stage. Installments belonging to these events are hidden from the
// /user listing (spec-096 / BUG5 requirement).
var excludedEventPhases = map[domain_event.EventoStatus]struct{}{
	"ORGANIZACAO":       {},
	"AGUARDANDO_ACEITE": {},
}

// ── ListUserInstallments ──────────────────────────────────────────────────────

// ListUserInstallments implements portin.ListUserInstallmentsUseCase.
//
// BUG5/spec-096 fix: searches by userId AND by all participationIds so that
// installments stored under a participation UUID are also returned.
// Installments from events in ORGANIZACAO / AGUARDANDO_ACEITE are excluded.
type ListUserInstallments struct {
	installments  portout.PaymentInstallmentRepository
	participantes portout.ParticipanteRepository
	eventos       portout.EventoRepository
}

// NewListUserInstallments creates a new ListUserInstallments use case.
func NewListUserInstallments(
	installments portout.PaymentInstallmentRepository,
	participantes portout.ParticipanteRepository,
	eventos portout.EventoRepository,
) *ListUserInstallments {
	return &ListUserInstallments{
		installments:  installments,
		participantes: participantes,
		eventos:       eventos,
	}
}

// Execute lists installments for a user, applying the BUG5 participationIds fix
// and filtering out events in planning phases.
func (uc *ListUserInstallments) Execute(ctx context.Context, userID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	// BUG5 fix: collect all participation IDs the user has.
	participationIDs, err := uc.participantes.FindParticipationIDsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user installments: get participations: %w", err)
	}

	// Fetch installments by userId AND participation IDs in one query.
	all, err := uc.installments.FindByUserOrParticipations(ctx, userID, participationIDs, statusFilter)
	if err != nil {
		return nil, fmt.Errorf("list user installments: find: %w", err)
	}

	if len(all) == 0 {
		return all, nil
	}

	// Collect unique event IDs to check their phases.
	eventIDs := uniqueEventIDs(all)
	excludedEvents := make(map[string]bool, len(eventIDs))
	for _, eid := range eventIDs {
		ev, err := uc.eventos.FindByID(ctx, eid)
		if err != nil {
			// Event not found or read error — do not filter (fail open).
			continue
		}
		if _, excluded := excludedEventPhases[ev.Status]; excluded {
			excludedEvents[eid] = true
		}
	}

	if len(excludedEvents) == 0 {
		return all, nil
	}

	filtered := make([]*domain.PaymentInstallment, 0, len(all))
	for _, inst := range all {
		if !excludedEvents[inst.EventID] {
			filtered = append(filtered, inst)
		}
	}
	return filtered, nil
}

// uniqueEventIDs returns the distinct event IDs referenced by the given installments.
func uniqueEventIDs(installments []*domain.PaymentInstallment) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, inst := range installments {
		if _, ok := seen[inst.EventID]; !ok {
			seen[inst.EventID] = struct{}{}
			ids = append(ids, inst.EventID)
		}
	}
	return ids
}

// ── ListInstallments ──────────────────────────────────────────────────────────

// ListInstallments implements portin.ListInstallmentsUseCase.
//
// EventID is required (returns apierr.BadRequest if empty).
// The requester must be a participant of the event (returns apierr.Forbidden if not).
// UserID and Status are optional filters.
type ListInstallments struct {
	installments  portout.PaymentInstallmentRepository
	participantes portout.ParticipanteRepository
}

// NewListInstallments creates a new ListInstallments use case.
func NewListInstallments(
	installments portout.PaymentInstallmentRepository,
	participantes portout.ParticipanteRepository,
) *ListInstallments {
	return &ListInstallments{
		installments:  installments,
		participantes: participantes,
	}
}

// Execute lists installments for an event, enforcing participant access control.
func (uc *ListInstallments) Execute(ctx context.Context, requesterID string, filter portin.ListInstallmentsFilter) ([]*domain.PaymentInstallment, error) {
	if filter.EventID == "" {
		return nil, apierr.BadRequest("eventId é obrigatório")
	}

	// Authorization: requester must be a participant of the event.
	isParticipant, err := uc.participantes.IsParticipantOfEvent(ctx, filter.EventID, requesterID)
	if err != nil {
		return nil, fmt.Errorf("list installments: check participant: %w", err)
	}
	if !isParticipant {
		return nil, apierr.Forbidden("acesso negado: você não é participante deste evento")
	}

	if filter.UserID != "" {
		// Specific participant filter.
		return uc.installments.FindByEventAndParticipant(ctx, filter.EventID, filter.UserID)
	}

	// No user filter: return all installments for the event.
	return uc.installments.FindByEvent(ctx, filter.EventID, filter.Status)
}
