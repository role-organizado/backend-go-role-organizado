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

type InviteLifecycleTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
}

func TestInviteLifecycle(t *testing.T) {
	suite.Run(t, new(InviteLifecycleTestSuite))
}

type stubExpirer struct{}

func (stubExpirer) Execute(_ context.Context, _ string) (*portin.ConviteResponse, error) {
	return &portin.ConviteResponse{}, nil
}

func (s *InviteLifecycleTestSuite) newEnv() *testsuite.TestWorkflowEnvironment {
	env := s.NewTestWorkflowEnvironment()
	acts := temporalactivity.NewInviteLifecycleActivities(stubExpirer{})
	env.RegisterActivity(acts)
	env.RegisterWorkflow(workflow.InviteLifecycleWorkflow)
	return env
}

// Test_Resolve resolves the approval on a resolveApproval signal.
func (s *InviteLifecycleTestSuite) Test_Resolve() {
	env := s.newEnv()

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("resolveApproval", workflow.InviteResolution{Status: "APPROVED", ResolverID: "org-1"})
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.InviteLifecycleWorkflow, workflow.InviteLifecycleInput{
		ApprovalID:              "appr-1",
		EventID:                 "evt-1",
		ReminderIntervalSeconds: 3600,
		ExpirationSeconds:       7200,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("RESOLVED", status)

	aval, err := env.QueryWorkflow("getCurrentApprovalStatus")
	s.Require().NoError(err)
	var approval string
	s.Require().NoError(aval.Get(&approval))
	s.Equal("APPROVED", approval)
}

// Test_Cancel cancels the approval on a cancelApproval signal.
func (s *InviteLifecycleTestSuite) Test_Cancel() {
	env := s.newEnv()

	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("cancelApproval", nil)
	}, time.Millisecond)

	env.ExecuteWorkflow(workflow.InviteLifecycleWorkflow, workflow.InviteLifecycleInput{
		ApprovalID:              "appr-1",
		ReminderIntervalSeconds: 3600,
		ExpirationSeconds:       7200,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("CANCELLED", status)
}

// Test_Expiration auto-declines via ProcessExpiration when no resolution arrives.
func (s *InviteLifecycleTestSuite) Test_Expiration() {
	env := s.newEnv()
	env.OnActivity("ProcessExpiration", mock.Anything, mock.Anything).Return(nil)

	// Expiration fires before the (far longer) reminder interval.
	env.ExecuteWorkflow(workflow.InviteLifecycleWorkflow, workflow.InviteLifecycleInput{
		ApprovalID:              "appr-1",
		ReminderIntervalSeconds: 3600,
		ExpirationSeconds:       1,
	})

	s.True(env.IsWorkflowCompleted())
	s.NoError(env.GetWorkflowError())

	val, err := env.QueryWorkflow("getWorkflowStatus")
	s.Require().NoError(err)
	var status string
	s.Require().NoError(val.Get(&status))
	s.Equal("EXPIRED", status)
}
