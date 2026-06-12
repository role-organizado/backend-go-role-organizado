package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// EventPublicationExecutionInput is the input for EventPublicationExecutionWorkflow.
type EventPublicationExecutionInput struct {
	DraftID     string `json:"draftId"`
	RequesterID string `json:"requesterId"`
}

// EventPublicationExecutionState is the observable state of a publication run.
type EventPublicationExecutionState struct {
	// Status is one of "CREATING_CORE", "MATERIALIZING_FINANCE", "FINALIZING",
	// "INITIALIZING_LIFECYCLE", "COMPLETED", or "FAILED".
	Status  string `json:"status"`
	DraftID string `json:"draftId"`
	EventID string `json:"eventId"`
}

// EventPublicationExecutionWorkflow publishes a draft into a live event through a
// staged pipeline: validate+create core, materialize finance, finalize, and
// (when the lifecycle-init version is active) bootstrap the event lifecycle.
//
// Versioning: gated on change IDs "event-publication-execution-v2" and
// "event-publication-lifecycle-init".
// Queries: getWorkflowStatus, getCurrentState
func EventPublicationExecutionWorkflow(ctx workflow.Context, input EventPublicationExecutionInput) (*activity.EventPublicationExecutionActivityResult, error) {
	_ = workflow.GetVersion(ctx, "event-publication-execution-v2", workflow.DefaultVersion, 2)
	lifecycleInit := workflow.GetVersion(ctx, "event-publication-lifecycle-init", workflow.DefaultVersion, 1)

	state := EventPublicationExecutionState{
		Status:  "CREATING_CORE",
		DraftID: input.DraftID,
	}

	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getCurrentState", func() (EventPublicationExecutionState, error) {
		return state, nil
	})

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	actInput := activity.EventPublicationExecutionActivityInput{
		DraftID:     input.DraftID,
		RequesterID: input.RequesterID,
	}

	// Stage 1 — validate and create the core event.
	var coreRes activity.EventPublicationExecutionActivityResult
	if err := workflow.ExecuteActivity(actCtx, "ValidateAndCreateCore", actInput).Get(ctx, &coreRes); err != nil {
		state.Status = "FAILED"
		return nil, fmt.Errorf("event publication validate/create core: %w", err)
	}
	state.EventID = coreRes.EventID
	actInput.EventID = coreRes.EventID

	// Stage 2 — materialize finance.
	state.Status = "MATERIALIZING_FINANCE"
	if err := workflow.ExecuteActivity(actCtx, "MaterializeFinance", actInput).Get(ctx, nil); err != nil {
		state.Status = "FAILED"
		return nil, fmt.Errorf("event publication materialize finance: %w", err)
	}

	// Stage 3 — finalize the publication.
	state.Status = "FINALIZING"
	if err := workflow.ExecuteActivity(actCtx, "FinalizePublication", actInput).Get(ctx, nil); err != nil {
		state.Status = "FAILED"
		return nil, fmt.Errorf("event publication finalize: %w", err)
	}

	// Stage 4 — bootstrap the event lifecycle (versioned addition).
	if lifecycleInit >= 1 {
		state.Status = "INITIALIZING_LIFECYCLE"
		if err := workflow.ExecuteActivity(actCtx, "InitializeEventLifecycle", actInput).Get(ctx, nil); err != nil {
			state.Status = "FAILED"
			return nil, fmt.Errorf("event publication initialize lifecycle: %w", err)
		}
	}

	state.Status = "COMPLETED"
	return &coreRes, nil
}
