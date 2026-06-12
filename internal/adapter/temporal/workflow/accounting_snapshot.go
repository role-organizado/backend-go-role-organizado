package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// accountingSnapshotVersionChange is the workflow.GetVersion change ID guarding
// the activity-timeout bump (30m → 60m). Mirrors the Java
// AccountingSnapshotWorkflowImpl Workflow.getVersion gate.
const accountingSnapshotVersionChange = "accounting-snapshot-aggregation-timeout"

// AccountingSnapshotInput is the input for AccountingSnapshotWorkflow.
type AccountingSnapshotInput struct {
	// DataInicio / DataFim bound the aggregation window (YYYY-MM-DD, optional).
	DataInicio string `json:"dataInicio,omitempty"`
	DataFim    string `json:"dataFim,omitempty"`
	// CorrelationID ties this run to the originating request (optional).
	CorrelationID string `json:"correlationId,omitempty"`
}

// AccountingSnapshotState holds the observable state of a snapshot run.
type AccountingSnapshotState struct {
	Status string                       `json:"status"`
	Result *activity.AccountingSnapshotResult `json:"result,omitempty"`
}

// AccountingSnapshotWorkflow aggregates platform accounting totals into a daily
// snapshot, mirroring the Java AccountingSnapshotWorkflowImpl.
//
// It uses Workflow.getVersion to gate a timeout bump and runs the
// GenerateAccountingSnapshot activity with a 30-minute (v0) / 60-minute (v1)
// StartToCloseTimeout and up to 3 retries.
//
// Queries: getWorkflowStatus, getCurrentState, getResult.
func AccountingSnapshotWorkflow(ctx workflow.Context, input AccountingSnapshotInput) error {
	state := AccountingSnapshotState{Status: "running"}

	if err := workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	}); err != nil {
		return fmt.Errorf("register getWorkflowStatus query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getCurrentState", func() (AccountingSnapshotState, error) {
		return state, nil
	}); err != nil {
		return fmt.Errorf("register getCurrentState query: %w", err)
	}
	if err := workflow.SetQueryHandler(ctx, "getResult", func() (*activity.AccountingSnapshotResult, error) {
		return state.Result, nil
	}); err != nil {
		return fmt.Errorf("register getResult query: %w", err)
	}

	// Versioned timeout: v0 → 30 minutes, v1 → 60 minutes.
	version := workflow.GetVersion(ctx, accountingSnapshotVersionChange, workflow.DefaultVersion, 1)
	timeout := 30 * time.Minute
	if version >= 1 {
		timeout = 60 * time.Minute
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: timeout,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	actInput := activity.AccountingSnapshotInput{
		DataInicio:    input.DataInicio,
		DataFim:       input.DataFim,
		CorrelationID: input.CorrelationID,
	}

	var result activity.AccountingSnapshotResult
	if err := workflow.ExecuteActivity(actCtx, "GenerateAccountingSnapshot", actInput).Get(actCtx, &result); err != nil {
		state.Status = "failed"
		return fmt.Errorf("accounting snapshot workflow: %w", err)
	}

	state.Result = &result
	state.Status = "completed"
	return nil
}
