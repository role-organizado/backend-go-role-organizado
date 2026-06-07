package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// SandboxWorkflowTestSuite validates the SandboxWorkflow using Temporal's test environment.
type SandboxWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
}

func (s *SandboxWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	act := activity.NewSandboxActivity()
	s.env.RegisterActivity(act)
}

func (s *SandboxWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func (s *SandboxWorkflowTestSuite) Test_SandboxWorkflow_ReturnsLoggedMessage() {
	s.env.ExecuteWorkflow(workflow.SandboxWorkflow, "hello world")

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result string
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("logged: hello world", result)
}

func (s *SandboxWorkflowTestSuite) Test_SandboxWorkflow_ReturnsLoggedMessage_EmptyInput() {
	s.env.ExecuteWorkflow(workflow.SandboxWorkflow, "")

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())

	var result string
	s.NoError(s.env.GetWorkflowResult(&result))
	s.Equal("logged: ", result)
}

// TestSandbox is the entry point for `go test -run TestSandbox`.
func TestSandbox(t *testing.T) {
	suite.Run(t, new(SandboxWorkflowTestSuite))
}
