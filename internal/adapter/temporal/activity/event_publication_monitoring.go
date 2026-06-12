package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// stuckExecutionFinder scans for stuck payment executions. Satisfied by
// event.FindStuckExecutions (FindStuckExecutionsUseCase).
type stuckExecutionFinder interface {
	Execute(ctx context.Context, in portin.FindStuckExecutionsInput) (*portin.FindStuckExecutionsResult, error)
}

// EventPublicationMonitoringActivityInput is the input for FindAndReportStuckExecutions.
type EventPublicationMonitoringActivityInput struct {
	StuckThresholdMinutes int `json:"stuckThresholdMinutes"`
	MaxResults            int `json:"maxResults"`
}

// EventPublicationMonitoringActivityResult is the outcome of a monitoring scan.
type EventPublicationMonitoringActivityResult struct {
	StuckCount int `json:"stuckCount"`
}

// EventPublicationMonitoringActivities holds dependencies for the monitoring activity.
type EventPublicationMonitoringActivities struct {
	finder stuckExecutionFinder
}

// NewEventPublicationMonitoringActivities creates a new EventPublicationMonitoringActivities instance.
func NewEventPublicationMonitoringActivities(finder stuckExecutionFinder) *EventPublicationMonitoringActivities {
	return &EventPublicationMonitoringActivities{finder: finder}
}

// FindAndReportStuckExecutions scans for stuck executions and reports the count.
func (a *EventPublicationMonitoringActivities) FindAndReportStuckExecutions(ctx context.Context, input EventPublicationMonitoringActivityInput) (EventPublicationMonitoringActivityResult, error) {
	logger := activity.GetLogger(ctx)

	res, err := a.finder.Execute(ctx, portin.FindStuckExecutionsInput{
		StuckThresholdMinutes: input.StuckThresholdMinutes,
		MaxResults:            input.MaxResults,
	})
	if err != nil {
		return EventPublicationMonitoringActivityResult{}, fmt.Errorf("find stuck executions: %w", err)
	}

	if res.StuckCount > 0 {
		logger.Warn("stuck event publication executions detected (native)", "stuckCount", res.StuckCount)
	} else {
		logger.Info("no stuck event publication executions detected (native)")
	}

	return EventPublicationMonitoringActivityResult{StuckCount: res.StuckCount}, nil
}
