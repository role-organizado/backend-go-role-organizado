package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	sdkworkflow "go.temporal.io/sdk/workflow"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

type EventPublicationMonitoringTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestEventPublicationMonitoring(t *testing.T) {
	suite.Run(t, new(EventPublicationMonitoringTestSuite))
}

type stubStuckFinder struct{}

func (stubStuckFinder) Execute(_ context.Context, _ portin.FindStuckExecutionsInput) (*portin.FindStuckExecutionsResult, error) {
	return &portin.FindStuckExecutionsResult{}, nil
}

func (s *EventPublicationMonitoringTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewEventPublicationMonitoringActivities(stubStuckFinder{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.EventPublicationMonitoringWorkflow)
	return env
}

// Test_ScansThenContinuesAsNew runs the bounded scan loop and continues-as-new.
func (s *EventPublicationMonitoringTestSuite) Test_ScansThenContinuesAsNew() {
	env := s.newEnv()
	env.OnActivity("FindAndReportStuckExecutions", mock.Anything, mock.Anything).Return(
		temporalactivity.EventPublicationMonitoringActivityResult{StuckCount: 5}, nil,
	)

	// After the first scan + sleep, the running state reflects the last scan.
	env.RegisterDelayedCallback(func() {
		val, err := env.QueryWorkflow("getCurrentState")
		s.Require().NoError(err)
		var state workflow.EventPublicationMonitoringState
		s.Require().NoError(val.Get(&state))
		s.Equal("RUNNING", state.Status)
		s.Equal(5, state.LastStuckCount)
		s.GreaterOrEqual(state.IterationCount, 1)
	}, 6*time.Minute)

	env.ExecuteWorkflow(workflow.EventPublicationMonitoringWorkflow, workflow.EventPublicationMonitoringInput{
		StuckThresholdMinutes: 30,
		MaxResults:            50,
	})

	s.True(env.IsWorkflowCompleted())

	// continueAsNew surfaces as a *ContinueAsNewError once the 100-iteration cap is hit.
	err := env.GetWorkflowError()
	s.Require().Error(err)
	var canErr *sdkworkflow.ContinueAsNewError
	s.True(errors.As(err, &canErr))
}
