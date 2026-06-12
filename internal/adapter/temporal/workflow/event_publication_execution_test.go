package workflow_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
	eventdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
)

type EventPublicationExecutionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestEventPublicationExecution(t *testing.T) {
	suite.Run(t, new(EventPublicationExecutionTestSuite))
}

type stubPublisher struct{}

func (stubPublisher) Execute(_ context.Context, _, _ string) (*eventdomain.Evento, error) {
	return &eventdomain.Evento{ID: "evt-9"}, nil
}

func (s *EventPublicationExecutionTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewEventPublicationExecutionActivities(stubPublisher{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.EventPublicationExecutionWorkflow)
	return env
}

// Test_FullPipeline runs all stages and completes with the created event ID.
func (s *EventPublicationExecutionTestSuite) Test_FullPipeline() {
	env := s.newEnv()
	env.OnActivity("ValidateAndCreateCore", mock.Anything, mock.Anything).Return(
		temporalactivity.EventPublicationExecutionActivityResult{EventID: "evt-9", Status: "CREATED"}, nil,
	)
	env.OnActivity("MaterializeFinance", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("FinalizePublication", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("InitializeEventLifecycle", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflow.EventPublicationExecutionWorkflow, workflow.EventPublicationExecutionInput{
		DraftID:     "draft-1",
		RequesterID: "org-1",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result temporalactivity.EventPublicationExecutionActivityResult
	s.Require().NoError(env.GetWorkflowResult(&result))
	s.Equal("evt-9", result.EventID)

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("COMPLETED", status)

	sval, err := env.QueryWorkflow("getCurrentState")
	s.Require().NoError(err)
	var state workflow.EventPublicationExecutionState
	s.Require().NoError(sval.Get(&state))
	s.Equal("evt-9", state.EventID)
}
