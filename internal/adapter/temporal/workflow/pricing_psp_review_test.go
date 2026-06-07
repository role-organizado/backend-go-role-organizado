package workflow_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	. "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// ── Test suite ────────────────────────────────────────────────────────────────

type PricingPspReviewWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	testEnv *testsuite.TestWorkflowEnvironment
}

func (s *PricingPspReviewWorkflowTestSuite) SetupTest() {
	s.testEnv = s.NewTestWorkflowEnvironment()
}

func (s *PricingPspReviewWorkflowTestSuite) TearDownTest() {
	s.testEnv.AssertExpectations(s.T())
}

func TestPricingPspReviewWorkflowSuite(t *testing.T) {
	suite.Run(t, new(PricingPspReviewWorkflowTestSuite))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestPricingPspReviewWorkflow_HappyPath verifies that:
//   - RunReview activity is called with the referenceDate
//   - workflow completes without error when the activity returns a completed result
//   - getWorkflowStatus query returns "COMPLETED" after execution
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReviewWorkflow_HappyPath() {
	const referenceDate = "2026-01-01"
	result := NewPricingPspReviewResultSuccess(150, 12, 4500)

	var a *PricingPspReviewActivities
	s.testEnv.OnActivity(a.RunReview, mock.Anything, referenceDate).
		Return(result, nil).
		Once()

	s.testEnv.ExecuteWorkflow(PricingPspReviewWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError())

	// Verify query state after completion
	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var status string
	s.NoError(v.Get(&status))
	s.Equal("COMPLETED", status)

	// Verify result query
	v, err = s.testEnv.QueryWorkflow("getResult")
	s.NoError(err)
	var gotResult *PricingPspReviewResult
	s.NoError(v.Get(&gotResult))
	s.NotNil(gotResult)
	s.True(gotResult.Completed)
	s.Equal(150, gotResult.ReviewedCount)
	s.Equal(12, gotResult.AppliedCount)
}

// TestPricingPspReviewWorkflow_ActivityError verifies that when RunReview returns
// an error the workflow propagates it and sets status to "FAILED".
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReviewWorkflow_ActivityError() {
	const referenceDate = "2026-01-02"

	var a *PricingPspReviewActivities
	s.testEnv.OnActivity(a.RunReview, mock.Anything, referenceDate).
		Return(PricingPspReviewResult{}, errors.New("PSP connection timeout")).
		Once()

	s.testEnv.ExecuteWorkflow(PricingPspReviewWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.Error(s.testEnv.GetWorkflowError(), "workflow should fail when activity fails")

	// Status should be FAILED
	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var status string
	s.NoError(v.Get(&status))
	s.Equal("FAILED", status)
}

// TestPricingPspReviewWorkflow_QueryGetWorkflowStatus verifies that the
// getWorkflowStatus query returns "RUNNING" while the activity is in progress.
//
// Technique: the activity mock is configured with After(2s) to simulate a
// long-running activity. A RegisterDelayedCallback at 1s fires while the
// activity is still executing and asserts status == "RUNNING".
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReviewWorkflow_QueryGetWorkflowStatus() {
	const referenceDate = "2026-01-03"
	result := NewPricingPspReviewResultSuccess(50, 3, 2000)

	var a *PricingPspReviewActivities
	// Simulated delay: activity "takes" 2 seconds of workflow time
	s.testEnv.OnActivity(a.RunReview, mock.Anything, referenceDate).
		After(2 * time.Second).
		Return(result, nil).
		Once()

	// Fire callback at t=1s (workflow is RUNNING, activity not yet complete)
	s.testEnv.RegisterDelayedCallback(func() {
		v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
		s.NoError(err, "query should succeed during workflow execution")
		var status string
		s.NoError(v.Get(&status))
		s.Equal("RUNNING", status, "workflow status must be RUNNING while activity is executing")
	}, time.Second)

	s.testEnv.ExecuteWorkflow(PricingPspReviewWorkflow, referenceDate)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.NoError(s.testEnv.GetWorkflowError())

	// After completion status must be COMPLETED
	v, err := s.testEnv.QueryWorkflow("getWorkflowStatus")
	s.NoError(err)
	var finalStatus string
	s.NoError(v.Get(&finalStatus))
	s.Equal("COMPLETED", finalStatus)
}
