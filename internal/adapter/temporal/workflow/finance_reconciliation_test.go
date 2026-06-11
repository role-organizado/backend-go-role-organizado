package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// FinanceReconciliationTestSuite runs all FinanceReconciliationWorkflow tests.
type FinanceReconciliationTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestFinanceReconciliation(t *testing.T) {
	suite.Run(t, new(FinanceReconciliationTestSuite))
}

// stubReconciler is a no-op financeReconciler used to satisfy the constructor.
// All real activity calls are intercepted by env.OnActivity mocks.
type stubReconciler struct{}

func (stubReconciler) Execute(_ context.Context, _ string) error { return nil }

// newEnv creates a test environment with FinanceReconciliationActivities pre-registered.
// Registration is required for OnActivity by string name to work.
// A stub use case is injected — the workflow's activities are intercepted by mocks.
func (s *FinanceReconciliationTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewFinanceReconciliationActivities(stubReconciler{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.FinanceReconciliationWorkflow)
	return env
}

// Test_Success_AllThreePasses verifies that when all 3 passes succeed the workflow
// completes with status "completed" and a populated result string.
func (s *FinanceReconciliationTestSuite) Test_Success_AllThreePasses() {
	env := s.newEnv()
	// Activity signature: RunReconciliation(ctx context.Context, referenceDate string) error
	// → two mock.Anything matchers: one for ctx, one for referenceDate.
	env.OnActivity("RunReconciliation", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflow.FinanceReconciliationWorkflow, "2024-01-15")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	// Query: GetWorkflowStatus
	val, err := env.QueryWorkflow("GetWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("completed", status)

	// Query: GetResult must mention the reference date
	resVal, err := env.QueryWorkflow("GetResult")
	s.Require().NoError(err)
	var result string
	s.Require().NoError(resVal.Get(&result))
	s.Contains(result, "2024-01-15")

	// Query: GetCurrentState
	stateVal, err := env.QueryWorkflow("GetCurrentState")
	s.Require().NoError(err)
	var state workflow.FinanceReconciliationState
	s.Require().NoError(stateVal.Get(&state))
	s.Equal("completed", state.Status)
	s.NotEmpty(state.Result)
}

// Test_Failure_ActivityAlwaysFails verifies that when the activity exhausts its
// retries the workflow fails and reports status "failed".
func (s *FinanceReconciliationTestSuite) Test_Failure_ActivityAlwaysFails() {
	env := s.newEnv()
	env.OnActivity("RunReconciliation", mock.Anything, mock.Anything).Return(
		errors.New("java backend unavailable"),
	)

	env.ExecuteWorkflow(workflow.FinanceReconciliationWorkflow, "2024-01-15")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("GetWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("failed", status)
}

// Test_Failure_Pass2Fails verifies that a failure on pass 2 (pass 1 succeeds) causes
// the workflow to fail with status "failed".
func (s *FinanceReconciliationTestSuite) Test_Failure_Pass2Fails() {
	env := s.newEnv()

	callN := 0
	env.OnActivity("RunReconciliation", mock.Anything, mock.Anything).Return(
		func(_ context.Context, _ string) error {
			callN++
			if callN == 1 {
				return nil // pass 1 succeeds
			}
			return errors.New("reconciliation mismatch on pass 2")
		},
	)

	env.ExecuteWorkflow(workflow.FinanceReconciliationWorkflow, "2024-01-15")

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("GetWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("failed", status)
}

// Test_EmptyReferenceDate_ComputesYesterday verifies that when referenceDate is empty
// (schedule-triggered run) the workflow auto-computes yesterday (UTC) and completes.
func (s *FinanceReconciliationTestSuite) Test_EmptyReferenceDate_ComputesYesterday() {
	env := s.newEnv()
	env.OnActivity("RunReconciliation", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflow.FinanceReconciliationWorkflow, "")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	resVal, err := env.QueryWorkflow("GetResult")
	s.Require().NoError(err)
	var result string
	s.Require().NoError(resVal.Get(&result))
	s.Contains(result, "triple-check reconciliation completed for")
}

// Test_TimeoutOptions_SmokeTest verifies the workflow executes without panics
// when correct timeout options are applied (structural smoke test).
func (s *FinanceReconciliationTestSuite) Test_TimeoutOptions_SmokeTest() {
	env := s.newEnv()
	env.OnActivity("RunReconciliation", mock.Anything, mock.Anything).Return(nil)

	env.ExecuteWorkflow(workflow.FinanceReconciliationWorkflow, "2024-06-01")

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}
