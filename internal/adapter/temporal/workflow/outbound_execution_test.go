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
	outbounddomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

type OutboundExecutionTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestOutboundExecution(t *testing.T) {
	suite.Run(t, new(OutboundExecutionTestSuite))
}

type stubApprover struct{}

func (stubApprover) Execute(_ context.Context, _ portin.ApproveOutboundRequestInput) (*outbounddomain.OutboundRequest, error) {
	return &outbounddomain.OutboundRequest{}, nil
}

type stubCallbackHandler struct{}

func (stubCallbackHandler) Execute(_ context.Context, _ portin.OutboundCallbackInput) (*outbounddomain.OutboundRequest, error) {
	return &outbounddomain.OutboundRequest{}, nil
}

func (s *OutboundExecutionTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewOutboundExecutionActivities(stubApprover{}, stubCallbackHandler{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.OutboundExecutionWorkflow)
	return env
}

func (s *OutboundExecutionTestSuite) input() workflow.OutboundExecutionInput {
	return workflow.OutboundExecutionInput{
		RequestID:      "req-1",
		EventID:        "evt-1",
		ApproverUserID: "org-1",
		ApprovalNotes:  "ok",
	}
}

// Test_CallbackReceived applies a provider callback and completes.
func (s *OutboundExecutionTestSuite) Test_CallbackReceived() {
	env := s.newEnv()
	env.OnActivity("PrepareExecution", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkAwaitingCallback", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("ApplyCallback", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("FinalizeExecution", mock.Anything, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("onProviderCallback", workflow.OutboundProviderCallback{
			Provider:        "ASAAS",
			ProviderStatus:  "DONE",
			Reason:          "completed",
			ProviderEventID: "evt-x",
		})
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.OutboundExecutionWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getCurrentState")
	s.Require().NoError(err)
	var state workflow.OutboundExecutionState
	s.Require().NoError(val.Get(&state))
	s.Equal("COMPLETED", state.Status)
	s.True(state.CallbackReceived)
	s.Equal("ASAAS", state.Provider)
}

// Test_Timeout handles a callback timeout and completes.
func (s *OutboundExecutionTestSuite) Test_Timeout() {
	env := s.newEnv()
	env.OnActivity("PrepareExecution", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("MarkAwaitingCallback", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("HandleTimeout", mock.Anything, mock.Anything).Return(nil)
	env.OnActivity("FinalizeExecution", mock.Anything, mock.Anything).Return(nil)

	// No callback signal — the 15-minute timer fires (auto-advanced by the test env).
	env.ExecuteWorkflow(workflow.OutboundExecutionWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getCurrentState")
	s.Require().NoError(err)
	var state workflow.OutboundExecutionState
	s.Require().NoError(val.Get(&state))
	s.Equal("COMPLETED", state.Status)
	s.True(state.TimedOut)
	s.False(state.CallbackReceived)
}
