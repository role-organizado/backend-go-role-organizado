package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// ParticipantLifecycleInput is the input for ParticipantLifecycleWorkflow.
type ParticipantLifecycleInput struct {
	EventID       string `json:"eventId"`
	ParticipantID string `json:"participantId"`
	RequesterID   string `json:"requesterId"`
	// Operation describes the change requested (e.g. "REMOVE_PARTICIPANT").
	Operation string `json:"operation"`
	// EstimatedInstallments / EstimatedRateios feed the calculation preview shown
	// to the organizer while the workflow awaits a decision.
	EstimatedInstallments int `json:"estimatedInstallments"`
	EstimatedRateios      int `json:"estimatedRateios"`
}

// ParticipantLifecycleState is the observable state of a participant lifecycle run.
type ParticipantLifecycleState struct {
	// Status is one of "AWAITING_DECISION", "EXECUTING", "COMPLETED",
	// "CANCELLED", or "FAILED".
	Status        string `json:"status"`
	EventID       string `json:"eventId"`
	ParticipantID string `json:"participantId"`
	Operation     string `json:"operation"`
	// Decision is "", "PROCEED", or "CANCEL".
	Decision string `json:"decision"`
}

// ParticipantCalculationPreview is the impact estimate exposed via the
// getCalculationPreview query while the workflow awaits a decision.
type ParticipantCalculationPreview struct {
	EventID                        string `json:"eventId"`
	ParticipantID                  string `json:"participantId"`
	EstimatedCancelledInstallments int    `json:"estimatedCancelledInstallments"`
	EstimatedRecalculatedRateios   int    `json:"estimatedRecalculatedRateios"`
}

// participantDecisionTimeout bounds how long the workflow waits for the organizer
// to decide before auto-cancelling the operation.
const participantDecisionTimeout = 24 * time.Hour

// ParticipantLifecycleWorkflow models the organizer-confirmed participant change
// flow. It blocks awaiting either a proceedWithChange or cancelOperation signal
// (or a decision timeout), then runs the ExecuteParticipantChange activity which
// cancels installments and recalculates rateios.
//
// Signals:  proceedWithChange, cancelOperation
// Queries:  getCurrentState, getWorkflowStatus, getCalculationPreview
func ParticipantLifecycleWorkflow(ctx workflow.Context, input ParticipantLifecycleInput) (*activity.ParticipantLifecycleActivityResult, error) {
	state := ParticipantLifecycleState{
		Status:        "AWAITING_DECISION",
		EventID:       input.EventID,
		ParticipantID: input.ParticipantID,
		Operation:     input.Operation,
	}
	preview := ParticipantCalculationPreview{
		EventID:                        input.EventID,
		ParticipantID:                  input.ParticipantID,
		EstimatedCancelledInstallments: input.EstimatedInstallments,
		EstimatedRecalculatedRateios:   input.EstimatedRateios,
	}

	_ = workflow.SetQueryHandler(ctx, "getCurrentState", func() (ParticipantLifecycleState, error) {
		return state, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getCalculationPreview", func() (ParticipantCalculationPreview, error) {
		return preview, nil
	})

	proceedCh := workflow.GetSignalChannel(ctx, "proceedWithChange")
	cancelCh := workflow.GetSignalChannel(ctx, "cancelOperation")

	proceed := false
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	sel := workflow.NewSelector(ctx)
	sel.AddReceive(proceedCh, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
		proceed = true
	})
	sel.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
		proceed = false
	})
	sel.AddFuture(workflow.NewTimer(timerCtx, participantDecisionTimeout), func(workflow.Future) {
		// Decision timeout — treat as cancellation.
		proceed = false
	})
	sel.Select(ctx)
	cancelTimer()

	if !proceed {
		state.Status = "CANCELLED"
		state.Decision = "CANCEL"
		return nil, nil
	}

	state.Decision = "PROCEED"
	state.Status = "EXECUTING"

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	actInput := activity.ParticipantLifecycleActivityInput{
		EventID:       input.EventID,
		ParticipantID: input.ParticipantID,
		RequesterID:   input.RequesterID,
		Reason:        input.Operation,
	}

	var result activity.ParticipantLifecycleActivityResult
	if err := workflow.ExecuteActivity(actCtx, "ExecuteParticipantChange", actInput).Get(ctx, &result); err != nil {
		state.Status = "FAILED"
		return nil, fmt.Errorf("participant lifecycle execution: %w", err)
	}

	state.Status = "COMPLETED"
	return &result, nil
}
