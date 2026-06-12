// Package activity contains Temporal activity implementations.
package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	financedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// financeRecalculator is the minimal contract needed by the participant
// recalculation activities. Satisfied by *finance.RecalculateFinanceSummary.
// Defined here as an interface so the activities can be unit-tested with a stub
// without depending on the concrete use case package.
//
// The recalculation is idempotent: running it to build a preview and again to
// apply produces the same persisted result, which keeps the human-in-the-loop
// confirmation step safe to retry.
type financeRecalculator interface {
	Execute(ctx context.Context, in portin.RecalculateFinanceSummaryInput) (*financedomain.FinanceSummary, error)
}

// CalculationPreview is the serialisable projection returned by CalculatePreview
// and exposed to operators via the workflow's getCalculationPreview query.
//
// It is defined in the activity package (not the workflow package) to avoid an
// import cycle — workflows import the activity package, never the reverse.
type CalculationPreview struct {
	EventID                string  `json:"eventId"`
	Goal                   int64   `json:"goal"`
	Collected              int64   `json:"collected"`
	ProgressPercentage     float64 `json:"progressPercentage"`
	AvailableForWithdrawal int64   `json:"availableForWithdrawal"`
	PendingWithdrawals     int64   `json:"pendingWithdrawals"`
}

// ParticipantRecalculationActivities groups the activities for the participant
// recalculation workflow. Register the struct with the worker; Temporal
// dispatches individual method calls by name.
type ParticipantRecalculationActivities struct {
	recalcUC financeRecalculator
}

// NewParticipantRecalculationActivities constructs ParticipantRecalculationActivities.
func NewParticipantRecalculationActivities(uc financeRecalculator) *ParticipantRecalculationActivities {
	return &ParticipantRecalculationActivities{recalcUC: uc}
}

// CalculatePreview recomputes the event's finance summary and returns it as a
// preview for operator confirmation. Registered as activity "CalculatePreview".
func (a *ParticipantRecalculationActivities) CalculatePreview(ctx context.Context, eventID string) (*CalculationPreview, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("calculating participant recalculation preview", "eventId", eventID)

	summary, err := a.recalcUC.Execute(ctx, portin.RecalculateFinanceSummaryInput{EventID: eventID})
	if err != nil {
		return nil, fmt.Errorf("calculate recalculation preview for event %s: %w", eventID, err)
	}

	return &CalculationPreview{
		EventID:                summary.EventID,
		Goal:                   summary.Goal,
		Collected:              summary.Collected,
		ProgressPercentage:     summary.ProgressPercentage,
		AvailableForWithdrawal: summary.AvailableForWithdrawal,
		PendingWithdrawals:     summary.PendingWithdrawals,
	}, nil
}

// ApplyRecalculation commits the recalculated finance summary for the event
// after the operator confirms via the proceedWithChange signal.
// Registered as activity "ApplyRecalculation".
func (a *ParticipantRecalculationActivities) ApplyRecalculation(ctx context.Context, eventID string) error {
	logger := activity.GetLogger(ctx)
	logger.Info("applying participant recalculation", "eventId", eventID)

	if _, err := a.recalcUC.Execute(ctx, portin.RecalculateFinanceSummaryInput{EventID: eventID}); err != nil {
		return fmt.Errorf("apply recalculation for event %s: %w", eventID, err)
	}
	return nil
}
