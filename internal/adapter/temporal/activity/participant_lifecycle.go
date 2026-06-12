// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// installmentCanceller cancels a participant's open (PENDING/OVERDUE) installments.
// Satisfied by participant.CancelParticipantInstallments.
type installmentCanceller interface {
	Execute(ctx context.Context, in portin.CancelParticipantInstallmentsInput) (int64, error)
}

// rateioRecalculator recomputes the rateio allocations of an event after a
// participant change. Satisfied by participant.RecalculateRateioAllocations.
type rateioRecalculator interface {
	Execute(ctx context.Context, in portin.RecalculateRateioAllocationsInput) (int64, error)
}

// ParticipantLifecycleActivityInput is the input for ExecuteParticipantChange.
type ParticipantLifecycleActivityInput struct {
	EventID       string `json:"eventId"`
	ParticipantID string `json:"participantId"`
	RequesterID   string `json:"requesterId"`
	// Reason is a human-readable description of why the change is being applied
	// (e.g. "PARTICIPANT_LEFT", "ORGANIZER_REMOVED").
	Reason string `json:"reason"`
}

// ParticipantLifecycleActivityResult is the outcome of a participant change.
type ParticipantLifecycleActivityResult struct {
	CancelledInstallments int64  `json:"cancelledInstallments"`
	RecalculatedRateios   int64  `json:"recalculatedRateios"`
	UpdatedInstallments   int64  `json:"updatedInstallments"`
	ExecutionReason       string `json:"executionReason"`
}

// ParticipantLifecycleActivities holds the dependencies for participant lifecycle activities.
type ParticipantLifecycleActivities struct {
	canceller    installmentCanceller
	recalculator rateioRecalculator
}

// NewParticipantLifecycleActivities creates a new ParticipantLifecycleActivities instance.
func NewParticipantLifecycleActivities(c installmentCanceller, r rateioRecalculator) *ParticipantLifecycleActivities {
	return &ParticipantLifecycleActivities{canceller: c, recalculator: r}
}

// ExecuteParticipantChange applies a confirmed participant change: it cancels the
// participant's open installments and recalculates the affected rateio allocations.
//
// Both steps are idempotent at the use-case layer, so the activity is safe to retry.
func (a *ParticipantLifecycleActivities) ExecuteParticipantChange(ctx context.Context, input ParticipantLifecycleActivityInput) (ParticipantLifecycleActivityResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("executing participant change (native)",
		"eventId", input.EventID,
		"participantId", input.ParticipantID,
		"reason", input.Reason,
	)

	cancelled, err := a.canceller.Execute(ctx, portin.CancelParticipantInstallmentsInput{
		EventID:       input.EventID,
		ParticipantID: input.ParticipantID,
		RequesterID:   input.RequesterID,
	})
	if err != nil {
		return ParticipantLifecycleActivityResult{}, fmt.Errorf("cancel participant installments: %w", err)
	}

	recalculated, err := a.recalculator.Execute(ctx, portin.RecalculateRateioAllocationsInput{
		EventID:       input.EventID,
		ParticipantID: input.ParticipantID,
		RequesterID:   input.RequesterID,
	})
	if err != nil {
		return ParticipantLifecycleActivityResult{}, fmt.Errorf("recalculate rateio allocations: %w", err)
	}

	reason := input.Reason
	if reason == "" {
		reason = "PARTICIPANT_CHANGE"
	}

	logger.Info("participant change completed (native)",
		"cancelledInstallments", cancelled,
		"recalculatedRateios", recalculated,
	)

	return ParticipantLifecycleActivityResult{
		CancelledInstallments: cancelled,
		RecalculatedRateios:   recalculated,
		UpdatedInstallments:   cancelled,
		ExecutionReason:       reason,
	}, nil
}
