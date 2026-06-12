package in

import "context"

// ─── Participant lifecycle inputs/outputs ──────────────────────────────────────

// RecalculateRateioAllocationsInput identifies the event whose rateio allocations
// must be recomputed after a participant change (e.g. a participant leaving the
// event), so the remaining participants absorb the redistributed cost.
type RecalculateRateioAllocationsInput struct {
	EventID       string
	ParticipantID string
	RequesterID   string
}

// RecalculateRateioAllocationsUseCase recomputes the per-participant allocations
// of every rateio attached to an event and returns the number of rateios that
// were recalculated.
//
// It is invoked by the ParticipantLifecycleWorkflow when an organizer proceeds
// with a participant change that affects cost sharing.
type RecalculateRateioAllocationsUseCase interface {
	Execute(ctx context.Context, in RecalculateRateioAllocationsInput) (int64, error)
}
