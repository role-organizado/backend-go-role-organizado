package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalact "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	temporalwf "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// mockPspCostReviewUC is a test double that satisfies in.RunPspCostReviewUseCase.
// Defined here to keep the test self-contained without importing port/in.
type mockPspCostReviewUC struct {
	err error
}

func (m *mockPspCostReviewUC) Execute(_ context.Context, _ string) error {
	return m.err
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

type PricingPspReviewWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *PricingPspReviewWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *PricingPspReviewWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPricingPspReview_Success verifies the happy path: activity succeeds →
// workflow completes without error.
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReview_Success() {
	act := temporalact.NewPricingPspReviewActivity(&mockPspCostReviewUC{err: nil})
	s.env.RegisterActivity(act)
	// OnActivity args include context (mock.Anything) + all non-ctx params.
	s.env.OnActivity(act.RunPspCostReview, mock.Anything, "2026-06-06").Return(nil)

	s.env.ExecuteWorkflow(temporalwf.PricingPspReviewWorkflow, "2026-06-06")

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestPricingPspReview_ActivityFailure verifies that an activity error propagates
// as a workflow error.
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReview_ActivityFailure() {
	actErr := errors.New("psp cost review failed")
	act := temporalact.NewPricingPspReviewActivity(&mockPspCostReviewUC{err: actErr})
	s.env.RegisterActivity(act)
	s.env.OnActivity(act.RunPspCostReview, mock.Anything, "2026-06-06").Return(actErr)

	s.env.ExecuteWorkflow(temporalwf.PricingPspReviewWorkflow, "2026-06-06")

	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// TestPricingPspReview_WorkflowID verifies the deterministic ID helper
// used for manual/triggered runs.
func (s *PricingPspReviewWorkflowTestSuite) TestPricingPspReview_WorkflowID() {
	id := temporalwf.PricingPspReviewPrimaryID("2026-06-06")
	s.Equal("pricing-psp-review-real-2026-06-06", id)
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func TestPricingPspReview(t *testing.T) {
	suite.Run(t, new(PricingPspReviewWorkflowTestSuite))
}
