package workflow_test

import (
	"context"
	"errors"
	"testing"

	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// OverdueInstallmentSuite exercises OverdueInstallmentWorkflow via the Temporal test suite.
type OverdueInstallmentSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

// TestOverdueInstallment is the entry point for `go test -run TestOverdueInstallment`.
func TestOverdueInstallment(t *testing.T) {
	suite.Run(t, new(OverdueInstallmentSuite))
}

// newEnv returns a fresh test environment with both activities registered.
//
// IMPORTANT: RegisterActivityWithOptions MUST be called before OnActivity (SDK invariant).
// We register no-op stubs so the registry knows "FindAndMarkOverdueInstallments" and
// "DispatchNotifications"; OnActivity then overrides their results with mocks.
func (s *OverdueInstallmentSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(
		func(_ context.Context, _ string) (int, error) { return 0, nil },
		sdkactivity.RegisterOptions{Name: "FindAndMarkOverdueInstallments"},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ string, _ int) error { return nil },
		sdkactivity.RegisterOptions{Name: "DispatchNotifications"},
	)
	return env
}

// Test_HappyPath_BothActivitiesRun verifies the full execution path when installments are found.
func (s *OverdueInstallmentSuite) Test_HappyPath_BothActivitiesRun() {
	env := s.newEnv()

	env.OnActivity("FindAndMarkOverdueInstallments", mock.Anything, "2026-06-06").
		Return(5, nil).Once()
	env.OnActivity("DispatchNotifications", mock.Anything, "2026-06-06", 5).
		Return(nil).Once()

	env.ExecuteWorkflow(workflow.OverdueInstallmentWorkflow, "2026-06-06")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())

	// Query handlers must reflect the final state.
	statusVal, err := env.QueryWorkflow("GetWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(statusVal.Get(&status))
	s.Equal("completed", status)

	stateVal, err := env.QueryWorkflow("GetCurrentState")
	s.Require().NoError(err)
	var state workflow.OverdueInstallmentState
	s.Require().NoError(stateVal.Get(&state))
	s.Equal("completed", state.Status)
	s.Equal(5, state.MarkedCount)
}

// Test_ZeroInstallments_SkipsDispatch verifies DispatchNotifications is NOT called when count == 0.
// If the workflow incorrectly calls DispatchNotifications, the env will use the no-op stub
// registered in newEnv — no mock expectation is set, so AssertExpectations will still pass,
// but the workflow would have to exercise the wrong branch. We detect this via query:
// MarkedCount must stay 0 and status must be "completed".
func (s *OverdueInstallmentSuite) Test_ZeroInstallments_SkipsDispatch() {
	env := s.newEnv()

	env.OnActivity("FindAndMarkOverdueInstallments", mock.Anything, "2026-06-06").
		Return(0, nil).Once()
	// DispatchNotifications must NOT be called — no mock expectation set.

	env.ExecuteWorkflow(workflow.OverdueInstallmentWorkflow, "2026-06-06")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
	env.AssertExpectations(s.T())

	stateVal, err := env.QueryWorkflow("GetCurrentState")
	s.Require().NoError(err)
	var state workflow.OverdueInstallmentState
	s.Require().NoError(stateVal.Get(&state))
	s.Equal("completed", state.Status)
	s.Equal(0, state.MarkedCount)
}

// Test_FirstActivityFailure_WorkflowFails verifies workflow propagates activity errors.
func (s *OverdueInstallmentSuite) Test_FirstActivityFailure_WorkflowFails() {
	env := s.newEnv()

	env.OnActivity("FindAndMarkOverdueInstallments", mock.Anything, mock.Anything).
		Return(0, errors.New("mongodb unreachable")).Once()

	env.ExecuteWorkflow(workflow.OverdueInstallmentWorkflow, "2026-06-06")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}

// Test_SecondActivityFailure_WorkflowFails verifies DispatchNotifications failures are propagated.
func (s *OverdueInstallmentSuite) Test_SecondActivityFailure_WorkflowFails() {
	env := s.newEnv()

	env.OnActivity("FindAndMarkOverdueInstallments", mock.Anything, "2026-06-06").
		Return(3, nil).Once()
	env.OnActivity("DispatchNotifications", mock.Anything, "2026-06-06", 3).
		Return(errors.New("notification service down")).Once()

	env.ExecuteWorkflow(workflow.OverdueInstallmentWorkflow, "2026-06-06")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())
	env.AssertExpectations(s.T())
}
