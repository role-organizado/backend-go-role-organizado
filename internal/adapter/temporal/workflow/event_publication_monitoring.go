package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// EventPublicationMonitoringInput is the input for EventPublicationMonitoringWorkflow.
type EventPublicationMonitoringInput struct {
	StuckThresholdMinutes int `json:"stuckThresholdMinutes"`
	MaxResults            int `json:"maxResults"`
	// IterationsSoFar carries the cumulative iteration count across continueAsNew.
	IterationsSoFar int `json:"iterationsSoFar"`
}

// EventPublicationMonitoringState is the observable state of the monitoring run.
type EventPublicationMonitoringState struct {
	// Status is always "RUNNING" for this perpetual singleton.
	Status         string `json:"status"`
	IterationCount int    `json:"iterationCount"`
	LastStuckCount int    `json:"lastStuckCount"`
}

const (
	// monitoringInterval is the sleep between monitoring scans (Workflow.sleep(5min)).
	monitoringInterval = 5 * time.Minute
	// monitoringMaxIterations bounds the scans per run before continueAsNew.
	monitoringMaxIterations = 100
	// defaultMonitoringThresholdMinutes / defaultMonitoringMaxResults apply when the
	// caller leaves the corresponding input fields at zero.
	defaultMonitoringThresholdMinutes = 30
	defaultMonitoringMaxResults       = 50
)

// EventPublicationMonitoringWorkflow is a perpetual singleton that periodically
// scans for stuck event-publication executions. It runs a scan, sleeps 5 minutes,
// and continues-as-new after 100 iterations to bound history growth.
//
// Singleton workflow ID: EventPublicationMonitoringPrimaryID().
// Queries: getWorkflowStatus, getCurrentState
func EventPublicationMonitoringWorkflow(ctx workflow.Context, input EventPublicationMonitoringInput) error {
	state := EventPublicationMonitoringState{
		Status:         "RUNNING",
		IterationCount: input.IterationsSoFar,
	}

	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getCurrentState", func() (EventPublicationMonitoringState, error) {
		return state, nil
	})

	threshold := input.StuckThresholdMinutes
	if threshold <= 0 {
		threshold = defaultMonitoringThresholdMinutes
	}
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMonitoringMaxResults
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	actInput := activity.EventPublicationMonitoringActivityInput{
		StuckThresholdMinutes: threshold,
		MaxResults:            maxResults,
	}

	for i := 0; i < monitoringMaxIterations; i++ {
		var res activity.EventPublicationMonitoringActivityResult
		if err := workflow.ExecuteActivity(actCtx, "FindAndReportStuckExecutions", actInput).Get(ctx, &res); err == nil {
			state.LastStuckCount = res.StuckCount
		}
		state.IterationCount++

		if err := workflow.Sleep(ctx, monitoringInterval); err != nil {
			return err
		}
	}

	return workflow.NewContinueAsNewError(ctx, EventPublicationMonitoringWorkflow, EventPublicationMonitoringInput{
		StuckThresholdMinutes: threshold,
		MaxResults:            maxResults,
		IterationsSoFar:       state.IterationCount,
	})
}
