package workflow_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	. "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// ── Test suite ────────────────────────────────────────────────────────────────

type FinanceReconciliationWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	testEnv *testsuite.TestWorkflowEnvironment
}

func (s *FinanceReconciliationWorkflowTestSuite) SetupTest() {
	s.testEnv = s.NewTestWorkflowEnvironment()
}

func (s *FinanceReconciliationWorkflowTestSuite) TearDownTest() {
	s.testEnv.AssertExpectations(s.T())
}

func TestFinanceReconciliationWorkflowSuite(t *testing.T) {
	suite.Run(t, new(FinanceReconciliationWorkflowTestSuite))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestFinanceReconciliationWorkflow_HappyPath verifies that:
//   - RunReconciliation activity is called with the referenceDate
//   - workflow completes without error when activity returns a completed result
//   - all query handlers return expected values after execution
func (s *FinanceReconciliationWorkflowTestSuite) TestFinanceReconciliationWorkflow_HappyPath() {
	const referenceDate = "2026-01-01"
	actResult := NewFinanceReconciliationResultSuccess(42, 3, 1, 8200)

	var a *FinanceReconciliationActivities
	s.testEnv.OnActivity(a.RunReconciliation, mock.Anything, referenceDate).
		Return(actResult, nil).
		Once()

	s.testEnv.ExecuteWorkflow(FinanceReconciliationWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError())

	// Status query
	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var status string
	s.NoError(v.Get(&status))
	s.Equal("COMPLETED", status)
}

// TestFinanceReconciliationWorkflow_ActivityError verifies that when the
// RunReconciliation activity returns an error the workflow propagates it
// and the status query returns "FAILED".
func (s *FinanceReconciliationWorkflowTestSuite) TestFinanceReconciliationWorkflow_ActivityError() {
	const referenceDate = "2026-01-02"

	var a *FinanceReconciliationActivities
	s.testEnv.OnActivity(a.RunReconciliation, mock.Anything, referenceDate).
		Return(FinanceReconciliationResult{}, errors.New("database connection lost")).
		Once()

	s.testEnv.ExecuteWorkflow(FinanceReconciliationWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.Error(s.testEnv.GetWorkflowError(), "workflow should fail when activity fails")

	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var status string
	s.NoError(v.Get(&status))
	s.Equal("FAILED", status)
}

// TestFinanceReconciliationWorkflow_Queries verifies that all three query
// handlers (getWorkflowStatus, getCurrentState, getResult) return consistent
// values after a successful workflow execution.
func (s *FinanceReconciliationWorkflowTestSuite) TestFinanceReconciliationWorkflow_Queries() {
	const referenceDate = "2026-01-03"
	actResult := NewFinanceReconciliationResultSuccess(100, 5, 2, 12000)

	var a *FinanceReconciliationActivities
	s.testEnv.OnActivity(a.RunReconciliation, mock.Anything, referenceDate).
		Return(actResult, nil).
		Once()

	s.testEnv.ExecuteWorkflow(FinanceReconciliationWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError())

	// getWorkflowStatus
	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var status string
	s.NoError(v.Get(&status))
	s.Equal("COMPLETED", status, "getWorkflowStatus should return COMPLETED")

	// getCurrentState
	v, err = s.testEnv.QueryWorkflow("getCurrentState")
	s.NoError(err)
	var state string
	s.NoError(v.Get(&state))
	s.Equal("COMPLETED", state, "getCurrentState should return COMPLETED")

	// getResult
	v, err = s.testEnv.QueryWorkflow("getResult")
	s.NoError(err)
	var result *FinanceReconciliationResult
	s.NoError(v.Get(&result))
	s.Require().NotNil(result, "getResult should return non-nil after completion")
	s.True(result.Completed)
	s.Equal(100, result.EventsChecked)
	s.Equal(5, result.Divergences)
	s.Equal(2, result.Critical)
	s.EqualValues(12000, result.DurationMs)
}
