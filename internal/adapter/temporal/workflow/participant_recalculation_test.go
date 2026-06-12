package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	financedomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// ParticipantRecalculationTestSuite runs all ParticipantRecalculationWorkflow tests.
type ParticipantRecalculationTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestParticipantRecalculation(t *testing.T) {
	suite.Run(t, new(ParticipantRecalculationTestSuite))
}

// stubRecalculator is a no-op financeRecalculator used only to satisfy the
// constructor; real activity calls are intercepted by env.OnActivity mocks.
type stubRecalculator struct{}

func (stubRecalculator) Execute(_ context.Context, _ portin.RecalculateFinanceSummaryInput) (*financedomain.FinanceSummary, error) {
	return &financedomain.FinanceSummary{}, nil
}

// newEnv creates a test environment with ParticipantRecalculationActivities
// pre-registered (required for OnActivity dispatch by string name).
func (s *ParticipantRecalculationTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewParticipantRecalculationActivities(stubRecalculator{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.ParticipantRecalculationWorkflow)
	return env
}

func (s *ParticipantRecalculationTestSuite) input() workflow.ParticipantRecalculationInput {
	return workflow.ParticipantRecalculationInput{EventID: "evt-1", UserID: "user-1"}
}

// Test_Proceed_AppliesRecalculation verifies that a proceedWithChange signal drives
// the workflow to COMPLETED with both activities executed.
func (s *ParticipantRecalculationTestSuite) Test_Proceed_AppliesRecalculation() {
	env := s.newEnv()
	env.OnActivity("CalculatePreview", mock.Anything, mock.Anything).Return(
		&temporalactivity.CalculationPreview{EventID: "evt-1", Goal: 10000, Collected: 4000},
		nil,
	)
	env.OnActivity("ApplyRecalculation", mock.Anything, mock.Anything).Return(nil)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalProceedWithChange, "ok")
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantRecalculationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.ParticipantRecalcStatusCompleted, status)

	previewVal, err := env.QueryWorkflow("getCalculationPreview")
	s.Require().NoError(err)
	var preview temporalactivity.CalculationPreview
	s.Require().NoError(previewVal.Get(&preview))
	s.Equal(int64(10000), preview.Goal)
}

// Test_Cancel_AbortsWithoutApplying verifies that a cancelOperation signal drives
// the workflow to CANCELLED without calling ApplyRecalculation.
func (s *ParticipantRecalculationTestSuite) Test_Cancel_AbortsWithoutApplying() {
	env := s.newEnv()
	env.OnActivity("CalculatePreview", mock.Anything, mock.Anything).Return(
		&temporalactivity.CalculationPreview{EventID: "evt-1"}, nil,
	)
	// ApplyRecalculation must NOT be called — no mock registered (panics if called).

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalCancelOperation, "abort")
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantRecalculationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.ParticipantRecalcStatusCancelled, status)
}

// Test_PreviewFails_WorkflowFails verifies a failure during the preview activity
// fails the workflow with status FAILED.
func (s *ParticipantRecalculationTestSuite) Test_PreviewFails_WorkflowFails() {
	env := s.newEnv()
	env.OnActivity("CalculatePreview", mock.Anything, mock.Anything).Return(
		(*temporalactivity.CalculationPreview)(nil),
		errors.New("finance summary repo unavailable"),
	)

	env.ExecuteWorkflow(workflow.ParticipantRecalculationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.ParticipantRecalcStatusFailed, status)
}

// Test_ApplyFails_WorkflowFails verifies a failure during apply (after a proceed
// signal) fails the workflow with status FAILED.
func (s *ParticipantRecalculationTestSuite) Test_ApplyFails_WorkflowFails() {
	env := s.newEnv()
	env.OnActivity("CalculatePreview", mock.Anything, mock.Anything).Return(
		&temporalactivity.CalculationPreview{EventID: "evt-1"}, nil,
	)
	env.OnActivity("ApplyRecalculation", mock.Anything, mock.Anything).Return(
		errors.New("apply failed"),
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalProceedWithChange, "ok")
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantRecalculationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.ParticipantRecalcStatusFailed, status)
}

// Test_ConfirmationTimeout_Cancels verifies that when no decision signal arrives
// before the confirmation timeout, the workflow cancels (fail-safe default).
func (s *ParticipantRecalculationTestSuite) Test_ConfirmationTimeout_Cancels() {
	env := s.newEnv()
	env.OnActivity("CalculatePreview", mock.Anything, mock.Anything).Return(
		&temporalactivity.CalculationPreview{EventID: "evt-1"}, nil,
	)
	// No signal sent — the test environment auto-advances the 24h timer.

	env.ExecuteWorkflow(workflow.ParticipantRecalculationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.ParticipantRecalcStatusCancelled, status)
}
