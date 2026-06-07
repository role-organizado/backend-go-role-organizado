package workflow_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	. "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// ─── Test suite ───────────────────────────────────────────────────────────────

type PaymentExpirationWorkflowSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env        *testsuite.TestWorkflowEnvironment
	activities *temporalactivity.PaymentActivities
}

func (s *PaymentExpirationWorkflowSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	// activities is nil — individual tests register mocks directly on env.
	s.activities = nil
}

func (s *PaymentExpirationWorkflowSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func TestExpirationWorkflowSuite(t *testing.T) {
	suite.Run(t, new(PaymentExpirationWorkflowSuite))
}

// ─── Test cases ───────────────────────────────────────────────────────────────

// TestExpirationWorkflow_ExpiresWithoutSignal verifies that when no paymentCompleted
// signal arrives before the expiry timer, the ExpireTransaction activity is called
// and the workflow completes successfully.
func (s *PaymentExpirationWorkflowSuite) TestExpirationWorkflow_ExpiresWithoutSignal() {
	var a *temporalactivity.PaymentActivities

	// Expect ExpireTransaction activity to be called exactly once.
	s.env.OnActivity(a.ExpireTransaction, mock.Anything, "tx-expiration-1").
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(PaymentExpirationWorkflow, PaymentExpirationInput{
		TransactionID:   "tx-expiration-1",
		ExpiresAtMillis: time.Now().Add(time.Hour).UnixMilli(), // timer skipped in tests
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestExpirationWorkflow_SignalBeforeTimer verifies that when a paymentCompleted
// signal arrives before the expiry timer, the ExpireTransaction activity is NOT called
// and the workflow completes cleanly (state = PAID).
func (s *PaymentExpirationWorkflowSuite) TestExpirationWorkflow_SignalBeforeTimer() {
	// Register the signal to fire before the timer.
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow(SignalPaymentCompleted, "webhook_confirmed")
	}, time.Millisecond) // fires before the 1-hour timer in test time

	// ExpireTransaction should NOT be called — if it is, the test panics (no mock).

	s.env.ExecuteWorkflow(PaymentExpirationWorkflow, PaymentExpirationInput{
		TransactionID:   "tx-expiration-2",
		ExpiresAtMillis: time.Now().Add(time.Hour).UnixMilli(),
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestExpirationWorkflow_AlreadyExpiredOnStart verifies that when ExpiresAtMillis is
// in the past, the workflow transitions to SKIPPED without calling any activity.
func (s *PaymentExpirationWorkflowSuite) TestExpirationWorkflow_AlreadyExpiredOnStart() {
	// ExpiresAt is in the past — no activity should be called.
	s.env.ExecuteWorkflow(PaymentExpirationWorkflow, PaymentExpirationInput{
		TransactionID:   "tx-expiration-3",
		ExpiresAtMillis: time.Now().Add(-time.Hour).UnixMilli(), // already expired
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestExpirationWorkflow_ActivityError verifies that when ExpireTransaction returns
// an error, the workflow returns an error.
func (s *PaymentExpirationWorkflowSuite) TestExpirationWorkflow_ActivityError() {
	var a *temporalactivity.PaymentActivities

	s.env.OnActivity(a.ExpireTransaction, mock.Anything, "tx-expiration-4").
		Return(errStub("DB unavailable")).
		Times(3) // MaximumAttempts=3

	s.env.ExecuteWorkflow(PaymentExpirationWorkflow, PaymentExpirationInput{
		TransactionID:   "tx-expiration-4",
		ExpiresAtMillis: time.Now().Add(time.Hour).UnixMilli(),
	})

	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// errStub is a test helper that returns a simple error.
func errStub(msg string) error {
	return errors.New(msg)
}
