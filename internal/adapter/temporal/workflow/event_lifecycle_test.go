package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

type EventLifecycleTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestEventLifecycle(t *testing.T) {
	suite.Run(t, new(EventLifecycleTestSuite))
}

type stubFaseAlterer struct{}

func (stubFaseAlterer) Execute(_ context.Context, _ portin.AlterarFaseInput) (*portin.AlterarFaseResult, error) {
	return &portin.AlterarFaseResult{}, nil
}

func (s *EventLifecycleTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewEventLifecycleActivities(stubFaseAlterer{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.EventLifecycleWorkflow)
	return env
}

// Test_FinalizedCompletes ends the workflow when a transition reaches FINALIZADO.
func (s *EventLifecycleTestSuite) Test_FinalizedCompletes() {
	env := s.newEnv()
	env.OnActivity("AlterarFaseEvento", mock.Anything, mock.Anything).Return(
		temporalactivity.EventLifecycleActivityResult{FaseAtual: "FINALIZADO"}, nil,
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("advanceToPreparacao", nil)
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.EventLifecycleWorkflow, workflow.EventLifecycleInput{
		EventoID: "evt-1",
		UserID:   "org-1",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("FINALIZED", status)
}

// Test_MultipleTransitions handles several signals before finalizing.
func (s *EventLifecycleTestSuite) Test_MultipleTransitions() {
	env := s.newEnv()

	callN := 0
	env.OnActivity("AlterarFaseEvento", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ temporalactivity.EventLifecycleActivityInput) (temporalactivity.EventLifecycleActivityResult, error) {
			callN++
			if callN >= 2 {
				return temporalactivity.EventLifecycleActivityResult{FaseAtual: "FINALIZADO"}, nil
			}
			return temporalactivity.EventLifecycleActivityResult{FaseAtual: "COLETA_PAGAMENTOS"}, nil
		},
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("releasePayments", nil)
	}, time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("advanceToPreparacao", nil)
	}, 2*time.Millisecond)

	env.ExecuteWorkflow(workflow.EventLifecycleWorkflow, workflow.EventLifecycleInput{
		EventoID: "evt-1",
		UserID:   "org-1",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getCurrentState")
	s.Require().NoError(err)
	var state workflow.EventLifecycleState
	s.Require().NoError(val.Get(&state))
	s.Equal("FINALIZED", state.Status)
	s.Equal(2, state.TransitionCount)
}
