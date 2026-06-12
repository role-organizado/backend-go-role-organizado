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

type ParticipantLifecycleTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestParticipantLifecycle(t *testing.T) {
	suite.Run(t, new(ParticipantLifecycleTestSuite))
}

// stubCanceller / stubRecalculator satisfy the activity's local interfaces.
type stubCanceller struct{}

func (stubCanceller) Execute(_ context.Context, _ portin.CancelParticipantInstallmentsInput) (int64, error) {
	return 0, nil
}

type stubRecalculator struct{}

func (stubRecalculator) Execute(_ context.Context, _ portin.RecalculateRateioAllocationsInput) (int64, error) {
	return 0, nil
}

func (s *ParticipantLifecycleTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewParticipantLifecycleActivities(stubCanceller{}, stubRecalculator{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.ParticipantLifecycleWorkflow)
	return env
}

func (s *ParticipantLifecycleTestSuite) input() workflow.ParticipantLifecycleInput {
	return workflow.ParticipantLifecycleInput{
		EventID:               "evt-1",
		ParticipantID:         "part-1",
		RequesterID:           "org-1",
		Operation:             "REMOVE_PARTICIPANT",
		EstimatedInstallments: 3,
		EstimatedRateios:      2,
	}
}

// Test_Proceed runs the activity after a proceedWithChange signal and completes.
func (s *ParticipantLifecycleTestSuite) Test_Proceed() {
	env := s.newEnv()
	env.OnActivity("ExecuteParticipantChange", mock.Anything, mock.Anything).Return(
		temporalactivity.ParticipantLifecycleActivityResult{
			CancelledInstallments: 3,
			RecalculatedRateios:   2,
			UpdatedInstallments:   3,
			ExecutionReason:       "REMOVE_PARTICIPANT",
		}, nil,
	)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("proceedWithChange", nil)
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantLifecycleWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	var result temporalactivity.ParticipantLifecycleActivityResult
	s.Require().NoError(env.GetWorkflowResult(&result))
	s.Equal(int64(3), result.CancelledInstallments)
	s.Equal(int64(2), result.RecalculatedRateios)

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("COMPLETED", status)
}

// Test_Cancel cancels the operation on a cancelOperation signal.
func (s *ParticipantLifecycleTestSuite) Test_Cancel() {
	env := s.newEnv()

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("cancelOperation", nil)
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantLifecycleWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("CANCELLED", status)
}

// Test_CalculationPreviewQuery verifies the preview is exposed while awaiting.
func (s *ParticipantLifecycleTestSuite) Test_CalculationPreviewQuery() {
	env := s.newEnv()

	env.RegisterDelayedCallback(func() {
		val, err := env.QueryWorkflow("getCalculationPreview")
		s.Require().NoError(err)
		var preview workflow.ParticipantCalculationPreview
		s.Require().NoError(val.Get(&preview))
		s.Equal(3, preview.EstimatedCancelledInstallments)
		s.Equal(2, preview.EstimatedRecalculatedRateios)
		env.SignalWorkflow("cancelOperation", nil)
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.ParticipantLifecycleWorkflow, s.input())
	s.True(env.IsWorkflowCompleted())
}
