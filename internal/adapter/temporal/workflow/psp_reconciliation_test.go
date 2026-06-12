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
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// PspReconciliationTestSuite runs all PspReconciliationWorkflow tests.
type PspReconciliationTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestPspReconciliation(t *testing.T) {
	suite.Run(t, new(PspReconciliationTestSuite))
}

// stubReconcilePsp satisfies the activity constructor; real activity calls are
// intercepted by env.OnActivity mocks.
type stubReconcilePsp struct{}

func (stubReconcilePsp) Execute(_ context.Context, _ portin.ReconcileFilter) (*portin.ReconcileResult, error) {
	return &portin.ReconcileResult{}, nil
}

// stubReportRepo is a no-op ReconciliationReportRepository.
type stubReportRepo struct{}

func (stubReportRepo) Save(_ context.Context, _ *temporalactivity.ReconciliationReport) error {
	return nil
}

func (s *PspReconciliationTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewPspReconciliationActivities(stubReconcilePsp{}, stubReportRepo{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.PspReconciliationWorkflow)
	return env
}

func (s *PspReconciliationTestSuite) input() workflow.PspReconciliationInput {
	return workflow.PspReconciliationInput{ScopeID: "global", ReferenceDate: "2026-06-10"}
}

// Test_Success runs the reconciliation activity and completes.
func (s *PspReconciliationTestSuite) Test_Success() {
	env := s.newEnv()
	env.OnActivity("RunPspReconciliation", mock.Anything, mock.Anything).Return(
		&temporalactivity.PspReconciliationResult{Checked: 10, Updated: 2, Failed: 0},
		nil,
	)

	env.ExecuteWorkflow(workflow.PspReconciliationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.PspReconStatusCompleted, status)

	stateVal, err := env.QueryWorkflow("getCurrentState")
	s.Require().NoError(err)
	var state workflow.PspReconciliationState
	s.Require().NoError(stateVal.Get(&state))
	s.Require().NotNil(state.Result)
	s.Equal(int64(10), state.Result.Checked)
}

// Test_Failure_ActivityFails fails the workflow when the activity errors.
func (s *PspReconciliationTestSuite) Test_Failure_ActivityFails() {
	env := s.newEnv()
	env.OnActivity("RunPspReconciliation", mock.Anything, mock.Anything).Return(
		(*temporalactivity.PspReconciliationResult)(nil),
		errors.New("asaas unavailable"),
	)

	env.ExecuteWorkflow(workflow.PspReconciliationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.PspReconStatusFailed, status)
}

// Test_Cancel_AbortsInFlightActivity verifies that a cancelReconciliation signal
// aborts a running activity and drives the workflow to CANCELLED.
func (s *PspReconciliationTestSuite) Test_Cancel_AbortsInFlightActivity() {
	env := s.newEnv()
	// Activity stays pending long enough for the cancel signal to interrupt it.
	env.OnActivity("RunPspReconciliation", mock.Anything, mock.Anything).
		Return(&temporalactivity.PspReconciliationResult{}, nil).
		After(time.Hour)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalCancelReconciliation, "stop")
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.PspReconciliationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.PspReconStatusCancelled, status)
}

// Test_PauseResume_StillCompletes verifies that pause followed by resume signals
// are accepted and the workflow still completes.
func (s *PspReconciliationTestSuite) Test_PauseResume_StillCompletes() {
	env := s.newEnv()
	env.OnActivity("RunPspReconciliation", mock.Anything, mock.Anything).
		Return(&temporalactivity.PspReconciliationResult{Checked: 3}, nil).
		After(time.Minute)

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalPauseReconciliation, "pause")
	}, time.Millisecond)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(workflow.SignalResumeReconciliation, "resume")
	}, 2*time.Millisecond)

	env.ExecuteWorkflow(workflow.PspReconciliationWorkflow, s.input())

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal(workflow.PspReconStatusCompleted, status)
}
