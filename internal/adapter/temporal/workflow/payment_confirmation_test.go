package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	. "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// ─── Test suite ───────────────────────────────────────────────────────────────

type PaymentConfirmationWorkflowSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *PaymentConfirmationWorkflowSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
}

func (s *PaymentConfirmationWorkflowSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func TestConfirmationWorkflowSuite(t *testing.T) {
	suite.Run(t, new(PaymentConfirmationWorkflowSuite))
}

// ─── Test cases ───────────────────────────────────────────────────────────────

// TestConfirmationWorkflow_Approved verifies that an APPROVED callback type
// executes the ConfirmPaymentApproved activity exactly once.
func (s *PaymentConfirmationWorkflowSuite) TestConfirmationWorkflow_Approved() {
	var a *temporalactivity.PaymentActivities

	s.env.OnActivity(a.ConfirmPaymentApproved, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(PaymentConfirmationWorkflow, PaymentConfirmationInput{
		ProviderTransactionID: "prov-tx-1",
		ProviderName:          "ASAAS",
		CallbackType:          "APPROVED",
		ProviderEventID:       "evt-1",
		EventType:             "PAYMENT_RECEIVED",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestConfirmationWorkflow_Failed verifies that a FAILED callback type
// executes the ConfirmPaymentFailed activity.
func (s *PaymentConfirmationWorkflowSuite) TestConfirmationWorkflow_Failed() {
	var a *temporalactivity.PaymentActivities

	s.env.OnActivity(a.ConfirmPaymentFailed, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(PaymentConfirmationWorkflow, PaymentConfirmationInput{
		ProviderTransactionID: "prov-tx-2",
		ProviderName:          "ASAAS",
		CallbackType:          "FAILED",
		FailureReason:         "REJECTED",
		ProviderEventID:       "evt-2",
		EventType:             "PAYMENT_REJECTED",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestConfirmationWorkflow_Cancelled verifies that a CANCELLED callback type
// executes the ConfirmPaymentCancelled activity.
func (s *PaymentConfirmationWorkflowSuite) TestConfirmationWorkflow_Cancelled() {
	var a *temporalactivity.PaymentActivities

	s.env.OnActivity(a.ConfirmPaymentCancelled, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(PaymentConfirmationWorkflow, PaymentConfirmationInput{
		ProviderTransactionID: "prov-tx-3",
		ProviderName:          "ASAAS",
		CallbackType:          "CANCELLED",
		ProviderEventID:       "evt-3",
		EventType:             "PAYMENT_CANCELLED",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

// TestConfirmationWorkflow_UnknownCallbackType verifies that an unknown callback type
// causes the workflow to fail with a non-retryable error.
func (s *PaymentConfirmationWorkflowSuite) TestConfirmationWorkflow_UnknownCallbackType() {
	s.env.ExecuteWorkflow(PaymentConfirmationWorkflow, PaymentConfirmationInput{
		ProviderTransactionID: "prov-tx-4",
		ProviderName:          "ASAAS",
		CallbackType:          "UNKNOWN_TYPE",
		ProviderEventID:       "evt-4",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// TestConfirmationWorkflow_ApprovedCaseInsensitive verifies that lowercase "approved"
// is treated the same as "APPROVED".
func (s *PaymentConfirmationWorkflowSuite) TestConfirmationWorkflow_ApprovedCaseInsensitive() {
	var a *temporalactivity.PaymentActivities

	s.env.OnActivity(a.ConfirmPaymentApproved, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	s.env.ExecuteWorkflow(PaymentConfirmationWorkflow, PaymentConfirmationInput{
		ProviderTransactionID: "prov-tx-5",
		ProviderName:          "ASAAS",
		CallbackType:          "approved", // lowercase
		ProviderEventID:       "evt-5",
		EventType:             "PAYMENT_RECEIVED",
	})

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}
