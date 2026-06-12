package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	accountingdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/accounting"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// AccountingSnapshotTestSuite runs all AccountingSnapshotWorkflow tests.
type AccountingSnapshotTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestAccountingSnapshot(t *testing.T) {
	suite.Run(t, new(AccountingSnapshotTestSuite))
}

// stubSnapshotGenerator satisfies the activity constructor; real activity calls
// are intercepted by env.OnActivity mocks.
type stubSnapshotGenerator struct{}

func (stubSnapshotGenerator) Execute(_ context.Context, _ portin.GenerateAccountingSnapshotInput) (*accountingdomain.Snapshot, error) {
	return &accountingdomain.Snapshot{}, nil
}

func (s *AccountingSnapshotTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewAccountingSnapshotActivities(stubSnapshotGenerator{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.AccountingSnapshotWorkflow)
	return env
}

// Test_Success aggregates and completes with a populated result.
func (s *AccountingSnapshotTestSuite) Test_Success() {
	env := s.newEnv()
	env.OnActivity("GenerateAccountingSnapshot", mock.Anything, mock.Anything).Return(
		&temporalactivity.AccountingSnapshotResult{
			SnapshotID:      "snap-1",
			TotalEventos:    5,
			TotalArrecadado: 120000,
		},
		nil,
	)

	env.ExecuteWorkflow(workflow.AccountingSnapshotWorkflow, workflow.AccountingSnapshotInput{
		DataInicio:    "2026-06-01",
		DataFim:       "2026-06-10",
		CorrelationID: "corr-1",
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("completed", status)

	resVal, err := env.QueryWorkflow("getResult")
	s.Require().NoError(err)
	var result temporalactivity.AccountingSnapshotResult
	s.Require().NoError(resVal.Get(&result))
	s.Equal("snap-1", result.SnapshotID)
	s.Equal(int64(5), result.TotalEventos)
}

// Test_Failure_ActivityFails fails the workflow when the aggregation activity errors.
func (s *AccountingSnapshotTestSuite) Test_Failure_ActivityFails() {
	env := s.newEnv()
	env.OnActivity("GenerateAccountingSnapshot", mock.Anything, mock.Anything).Return(
		(*temporalactivity.AccountingSnapshotResult)(nil),
		errors.New("mongo aggregate failed"),
	)

	env.ExecuteWorkflow(workflow.AccountingSnapshotWorkflow, workflow.AccountingSnapshotInput{})

	s.True(env.IsWorkflowCompleted())
	s.Error(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("failed", status)
}

// Test_EmptyWindow_Completes verifies an unbounded window run completes.
func (s *AccountingSnapshotTestSuite) Test_EmptyWindow_Completes() {
	env := s.newEnv()
	env.OnActivity("GenerateAccountingSnapshot", mock.Anything, mock.Anything).Return(
		&temporalactivity.AccountingSnapshotResult{SnapshotID: "snap-2"}, nil,
	)

	env.ExecuteWorkflow(workflow.AccountingSnapshotWorkflow, workflow.AccountingSnapshotInput{})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())
}
