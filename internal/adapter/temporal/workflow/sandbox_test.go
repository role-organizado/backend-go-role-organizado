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

type SandboxWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	testEnv *testsuite.TestWorkflowEnvironment
}

func (s *SandboxWorkflowTestSuite) SetupTest() {
	s.testEnv = s.NewTestWorkflowEnvironment()
}

func (s *SandboxWorkflowTestSuite) TearDownTest() {
	s.testEnv.AssertExpectations(s.T())
}

func TestSandboxWorkflowSuite(t *testing.T) {
	suite.Run(t, new(SandboxWorkflowTestSuite))
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestSandboxWorkflow_HappyPath verifies that the sandbox workflow:
//   - executes with a message parameter
//   - calls the EnviarLogAssincrono activity exactly once with the given message
//   - completes without error
func (s *SandboxWorkflowTestSuite) TestSandboxWorkflow_HappyPath() {
	const message = "Festa Junina 2026"

	var a *SandboxActivities
	s.testEnv.OnActivity(a.EnviarLogAssincrono, mock.Anything, message).
		Return(nil).
		Once()

	s.testEnv.ExecuteWorkflow(SandboxWorkflow, message)

	s.True(s.testEnv.IsWorkflowCompleted(), "workflow should have completed")
	s.NoError(s.testEnv.GetWorkflowError(), "workflow should complete without error")
}

// TestSandboxWorkflow_ActivityError verifies that if the activity fails the
// workflow propagates the error back to the caller.
func (s *SandboxWorkflowTestSuite) TestSandboxWorkflow_ActivityError() {
	const message = "bad-event"

	var a *SandboxActivities
	s.testEnv.OnActivity(a.EnviarLogAssincrono, mock.Anything, message).
		Return(errors.New("activity failure: downstream service unavailable")).
		Once()

	s.testEnv.ExecuteWorkflow(SandboxWorkflow, message)

	s.True(s.testEnv.IsWorkflowCompleted())
	s.Error(s.testEnv.GetWorkflowError(), "workflow should propagate the activity error")
}
