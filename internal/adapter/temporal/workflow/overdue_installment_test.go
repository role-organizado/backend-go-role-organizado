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

type OverdueInstallmentWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	testEnv *testsuite.TestWorkflowEnvironment
}

func (s *OverdueInstallmentWorkflowTestSuite) SetupTest() {
	s.testEnv = s.NewTestWorkflowEnvironment()
}

func (s *OverdueInstallmentWorkflowTestSuite) TearDownTest() {
	s.testEnv.AssertExpectations(s.T())
}

func TestOverdueInstallmentWorkflowSuite(t *testing.T) {
	suite.Run(t, new(OverdueInstallmentWorkflowTestSuite))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestOverdueInstallmentWorkflow_HappyPath verifies that when count > 0:
//   - FindAndMarkOverdueInstallments is called and returns a positive count
//   - DispatchNotifications IS called with the returned count
//   - workflow completes without error
//   - result contains correct installments/notifications counts
func (s *OverdueInstallmentWorkflowTestSuite) TestOverdueInstallmentWorkflow_HappyPath() {
	const referenceDate = "2026-01-01"
	const overdueCount = 7

	var a *OverdueInstallmentActivities

	// Both activities must be called exactly once
	s.testEnv.OnActivity(a.FindAndMarkOverdueInstallments, mock.Anything, referenceDate).
		Return(overdueCount, nil).
		Once()

	s.testEnv.OnActivity(a.DispatchNotifications, mock.Anything, overdueCount).
		Return(nil).
		Once()

	s.testEnv.ExecuteWorkflow(OverdueInstallmentWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError())

	// Verify result
	var result *OverdueInstallmentResult
	s.NoError(s.testEnv.GetWorkflowResult(&result))
	s.Require().NotNil(result)
	s.True(result.Completed)
	s.Equal(overdueCount, result.InstallmentsProcessed)
	s.Equal(overdueCount, result.NotificationsSent)
}

// TestOverdueInstallmentWorkflow_ZeroCount verifies that when count == 0:
//   - FindAndMarkOverdueInstallments is called and returns 0
//   - DispatchNotifications is NOT called (no overdue installments)
//   - workflow completes without error
//   - result has zero notifications sent
//
// The implicit assertion that DispatchNotifications is NOT called relies on
// the test environment returning an error for any unmocked activity call,
// which would cause the workflow to fail (caught by s.NoError below).
func (s *OverdueInstallmentWorkflowTestSuite) TestOverdueInstallmentWorkflow_ZeroCount() {
	const referenceDate = "2026-01-02"

	var a *OverdueInstallmentActivities

	// Only FindAndMarkOverdueInstallments is mocked; DispatchNotifications is not.
	// If the workflow incorrectly calls DispatchNotifications, it will fail.
	s.testEnv.OnActivity(a.FindAndMarkOverdueInstallments, mock.Anything, referenceDate).
		Return(0, nil).
		Once()

	s.testEnv.ExecuteWorkflow(OverdueInstallmentWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError(), "workflow must complete successfully with zero count")

	var result *OverdueInstallmentResult
	s.NoError(s.testEnv.GetWorkflowResult(&result))
	s.Require().NotNil(result)
	s.True(result.Completed)
	s.Equal(0, result.InstallmentsProcessed)
	s.Equal(0, result.NotificationsSent, "no notifications should be sent when count == 0")
}

// TestOverdueInstallmentWorkflow_Queries verifies that all three query handlers
// (getWorkflowStatus, getCurrentState, getResult) return consistent values
// after a successful workflow execution.
func (s *OverdueInstallmentWorkflowTestSuite) TestOverdueInstallmentWorkflow_Queries() {
	const referenceDate = "2026-01-03"
	const overdueCount = 3

	var a *OverdueInstallmentActivities
	s.testEnv.OnActivity(a.FindAndMarkOverdueInstallments, mock.Anything, referenceDate).
		Return(overdueCount, nil).
		Once()
	s.testEnv.OnActivity(a.DispatchNotifications, mock.Anything, overdueCount).
		Return(nil).
		Once()

	s.testEnv.ExecuteWorkflow(OverdueInstallmentWorkflow, referenceDate)

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
	var result *OverdueInstallmentResult
	s.NoError(v.Get(&result))
	s.Require().NotNil(result, "getResult should return non-nil after completion")
	s.True(result.Completed)
	s.Equal(overdueCount, result.InstallmentsProcessed)
	s.Equal(overdueCount, result.NotificationsSent)
}

// TestOverdueInstallmentWorkflow_ActivityError verifies that when
// FindAndMarkOverdueInstallments fails, the workflow propagates the error.
func (s *OverdueInstallmentWorkflowTestSuite) TestOverdueInstallmentWorkflow_ActivityError() {
	const referenceDate = "2026-01-04"

	var a *OverdueInstallmentActivities
	s.testEnv.OnActivity(a.FindAndMarkOverdueInstallments, mock.Anything, referenceDate).
		Return(0, errors.New("mongodb connection refused")).
		Once()

	s.testEnv.ExecuteWorkflow(OverdueInstallmentWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.Error(s.testEnv.GetWorkflowError(), "workflow should fail when first activity fails")
}
