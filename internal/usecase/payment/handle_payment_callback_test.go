package payment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	notificationdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/notification"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Mocks ───────────────────────────────────────────────────────────────────

type mockWebhookRepo struct{ mock.Mock }

func (m *mockWebhookRepo) ExistsByProviderAndEventID(ctx context.Context, provider, eventID string) (bool, error) {
	args := m.Called(ctx, provider, eventID)
	return args.Bool(0), args.Error(1)
}

func (m *mockWebhookRepo) SaveUnique(ctx context.Context, e *domain.ProcessedWebhookEvent) error {
	args := m.Called(ctx, e)
	return args.Error(0)
}

// Compile-time assertions.
var _ portout.ProcessedWebhookEventRepository = (*mockWebhookRepo)(nil)

// ── mockConfirmationWriter ────────────────────────────────────────────────────

type mockConfirmationWriter struct{ mock.Mock }

func (m *mockConfirmationWriter) AppendPaymentConfirmation(ctx context.Context, tx *domain.PaymentTransaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

// Compile-time assertion (implements ucpayment.PaymentConfirmationWriter indirectly via duck typing).

// ── mockCreateNotif ───────────────────────────────────────────────────────────

type mockCreateNotif struct{ mock.Mock }

func (m *mockCreateNotif) Execute(ctx context.Context, in portin.CreateNotificacaoInput) (*notificationdomain.Notificacao, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*notificationdomain.Notificacao), args.Error(1)
}

var _ portin.CreateNotificacaoUseCase = (*mockCreateNotif)(nil)

// ─── Helpers ─────────────────────────────────────────────────────────────────

func pendingTransaction(id, userID, providerTxID string) *domain.PaymentTransaction {
	return &domain.PaymentTransaction{
		ID:                    id,
		UserID:                userID,
		EventID:               "evt-1",
		InstallmentIDs:        []string{"inst-1"},
		AmountCents:           10000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderAsaas,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: providerTxID,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

func callbackPayload(eventID, providerTxID, status string) portin.PaymentCallbackPayload {
	return portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       eventID,
		ProviderTransactionID: providerTxID,
		EventType:             "PAYMENT_RECEIVED",
		NewStatus:             status,
	}
}

func newCallbackUC(txRepo *mockTxRepo, instRepo *mockInstallmentRepo, webhookRepo *mockWebhookRepo, subledger *mockConfirmationWriter, notif *mockCreateNotif) *ucpayment.HandlePaymentCallback {
	return ucpayment.NewHandlePaymentCallback(txRepo, instRepo, webhookRepo, nil, subledger, notif)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestHandlePaymentCallback_IdempotencyPreCheck verifies that when an event has
// already been processed, Execute returns nil without touching the transaction.
func TestHandlePaymentCallback_IdempotencyPreCheck(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-already").
		Return(true, nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-already", "pay-1", "RECEIVED"))

	require.NoError(t, err)
	txRepo.AssertNotCalled(t, "FindByProviderTransactionID")
	webhookRepo.AssertExpectations(t)
}

// TestHandlePaymentCallback_NonTerminalStatus_PENDING verifies PENDING is a no-op.
func TestHandlePaymentCallback_NonTerminalStatus_PENDING(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-2").
		Return(false, nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-2", "pay-2", "PENDING"))

	require.NoError(t, err)
	// SaveUnique must NOT be called for non-terminal statuses.
	webhookRepo.AssertNotCalled(t, "SaveUnique")
	txRepo.AssertNotCalled(t, "FindByProviderTransactionID")
}

// TestHandlePaymentCallback_NonTerminalStatus_AWAITING verifies AWAITING_PAYMENT is a no-op.
func TestHandlePaymentCallback_NonTerminalStatus_AWAITING(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-awaiting").
		Return(false, nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-awaiting", "pay-3", "AWAITING_PAYMENT"))

	require.NoError(t, err)
	webhookRepo.AssertNotCalled(t, "SaveUnique")
}

// TestHandlePaymentCallback_Approved_MarksCompletedAndNotifies verifies:
//   - Transaction status → COMPLETED
//   - Installments → PAID via MarkPaidBatch
//   - Notification created for user
//   - Subledger confirmation written
func TestHandlePaymentCallback_Approved_MarksCompletedAndNotifies(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	tx := pendingTransaction("tx-1", "user-1", "pay-asaas-1")
	paidInst := &domain.PaymentInstallment{
		ID: "inst-1", Status: domain.InstallmentStatusPaid,
		EventID: "evt-1", ParticipantID: "user-1", AmountCents: 10000,
	}

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-approved").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-asaas-1").
		Return(tx, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).
		Return(nil)
	instRepo.On("MarkPaidBatch", mock.Anything, []string{"inst-1"}, "tx-1",
		mock.AnythingOfType("time.Time"), "PIX", "pay-asaas-1").
		Return(nil)
	// FindByIDs is only called when allocationSvc != nil. newCallbackUC passes nil
	// for allocationSvc, so FindByIDs is NOT expected here.
	_ = paidInst // used in the full integration test instead
	subledger.On("AppendPaymentConfirmation", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).
		Return(nil)
	notif.On("Execute", mock.Anything, mock.MatchedBy(func(in portin.CreateNotificacaoInput) bool {
		return in.UsuarioID == "user-1" && in.Tipo == notificationdomain.TipoNotificacaoPagamento
	})).Return(&notificationdomain.Notificacao{ID: "notif-1"}, nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-approved", "pay-asaas-1", "RECEIVED"))

	require.NoError(t, err)
	// Transaction must have been updated to COMPLETED.
	require.Equal(t, domain.TransactionStatusCompleted, tx.Status)
	require.NotNil(t, tx.CompletedAt)

	webhookRepo.AssertExpectations(t)
	txRepo.AssertExpectations(t)
	instRepo.AssertExpectations(t)
	subledger.AssertExpectations(t)
	notif.AssertExpectations(t)
}

// TestHandlePaymentCallback_Approved_AlreadyCompleted verifies idempotency when
// the transaction is already in a terminal state.
func TestHandlePaymentCallback_Approved_AlreadyCompleted(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	completedAt := time.Now()
	tx := pendingTransaction("tx-2", "user-2", "pay-2")
	tx.Status = domain.TransactionStatusCompleted
	tx.CompletedAt = &completedAt

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-dup").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-2").
		Return(tx, nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-dup", "pay-2", "RECEIVED"))

	require.NoError(t, err)
	txRepo.AssertNotCalled(t, "Update")
	instRepo.AssertNotCalled(t, "MarkPaidBatch")
}

// TestHandlePaymentCallback_Failed_MarksFailedWithReason verifies:
//   - Transaction status → FAILED
//   - FailureReason set to the status string
func TestHandlePaymentCallback_Failed_MarksFailedWithReason(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	tx := pendingTransaction("tx-3", "user-3", "pay-3")

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-failed").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-3").
		Return(tx, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).
		Return(nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-failed", "pay-3", "OVERDUE"))

	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusFailed, tx.Status)
	assert.Equal(t, "OVERDUE", tx.FailureReason)
}

// TestHandlePaymentCallback_Cancelled verifies CANCELLED status sets the correct state.
func TestHandlePaymentCallback_Cancelled(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	tx := pendingTransaction("tx-4", "user-4", "pay-4")

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-cancelled").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-4").
		Return(tx, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).
		Return(nil)

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-cancelled", "pay-4", "CANCELLED"))

	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCancelled, tx.Status)
}

// TestHandlePaymentCallback_RaceCondition_SaveUniqueReturnsErrAlreadyProcessed
// verifies that a concurrent delivery that reaches SaveUnique after the pre-check
// is treated as a silent no-op (returns nil).
func TestHandlePaymentCallback_RaceCondition_SaveUniqueReturnsErrAlreadyProcessed(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-race").
		Return(false, nil) // pre-check passes (not yet persisted)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(portout.ErrAlreadyProcessed) // concurrent goroutine already saved it

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-race", "pay-race", "RECEIVED"))

	require.NoError(t, err)
	// No transaction operations must have been triggered.
	txRepo.AssertNotCalled(t, "FindByProviderTransactionID")
	txRepo.AssertNotCalled(t, "Update")
}

// TestHandlePaymentCallback_TransactionNotFound verifies an error is returned
// (and wrapped) when the transaction cannot be found for an approved event.
func TestHandlePaymentCallback_TransactionNotFound(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-notfound").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-missing").
		Return(nil, apierr.NotFound("payment_transaction", "pay-missing"))

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-notfound", "pay-missing", "RECEIVED"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "find transaction (approved)")
}

// TestHandlePaymentCallback_Approved_NotificationNonFatal verifies that a
// notification error does NOT fail the callback (non-fatal).
func TestHandlePaymentCallback_Approved_NotificationNonFatal(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)
	subledger := new(mockConfirmationWriter)
	notif := new(mockCreateNotif)

	tx := pendingTransaction("tx-5", "user-5", "pay-5")

	webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, "ASAAS", "evt-notif-fail").
		Return(false, nil)
	webhookRepo.On("SaveUnique", mock.Anything, mock.AnythingOfType("*payment.ProcessedWebhookEvent")).
		Return(nil)
	txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-5").
		Return(tx, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).
		Return(nil)
	instRepo.On("MarkPaidBatch", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	instRepo.On("FindByIDs", mock.Anything, mock.Anything).
		Return([]*domain.PaymentInstallment{}, nil)
	subledger.On("AppendPaymentConfirmation", mock.Anything, mock.Anything).
		Return(nil)
	notif.On("Execute", mock.Anything, mock.Anything).
		Return(nil, errors.New("notification service unavailable")) // non-fatal error

	uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
	err := uc.Execute(context.Background(), callbackPayload("evt-notif-fail", "pay-5", "RECEIVED"))

	// The use case must succeed even when notification fails.
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, tx.Status)
}

// TestHandlePaymentCallback_StatusMapping verifies the canonical mapping table.
func TestHandlePaymentCallback_StatusMapping(t *testing.T) {
	cases := []struct {
		status      string
		expectFinal domain.TransactionStatus
	}{
		{"RECEIVED", domain.TransactionStatusCompleted},
		{"CONFIRMED", domain.TransactionStatusCompleted},
		{"APPROVED", domain.TransactionStatusCompleted},
		{"COMPLETED", domain.TransactionStatusCompleted},
		{"OVERDUE", domain.TransactionStatusFailed},
		{"REJECTED", domain.TransactionStatusFailed},
		{"FAILED", domain.TransactionStatusFailed},
		{"CANCELLED", domain.TransactionStatusCancelled},
		{"REFUNDED", domain.TransactionStatusCancelled},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			txRepo := new(mockTxRepo)
			instRepo := new(mockInstallmentRepo)
			webhookRepo := new(mockWebhookRepo)
			subledger := new(mockConfirmationWriter)
			notif := new(mockCreateNotif)

			tx := pendingTransaction("tx-map", "user-map", "pay-map")

			webhookRepo.On("ExistsByProviderAndEventID", mock.Anything, mock.Anything, mock.Anything).
				Return(false, nil)
			webhookRepo.On("SaveUnique", mock.Anything, mock.Anything).
				Return(nil)
			txRepo.On("FindByProviderTransactionID", mock.Anything, "pay-map").
				Return(tx, nil)
			txRepo.On("Update", mock.Anything, mock.Anything).
				Return(nil)

			// Approved path extras.
			instRepo.On("MarkPaidBatch", mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
			instRepo.On("FindByIDs", mock.Anything, mock.Anything).
				Return([]*domain.PaymentInstallment{}, nil).Maybe()
			subledger.On("AppendPaymentConfirmation", mock.Anything, mock.Anything).
				Return(nil).Maybe()
			notif.On("Execute", mock.Anything, mock.Anything).
				Return(nil, nil).Maybe()

			uc := newCallbackUC(txRepo, instRepo, webhookRepo, subledger, notif)
			err := uc.Execute(context.Background(), callbackPayload("evt-"+tc.status, "pay-map", tc.status))

			require.NoError(t, err)
			assert.Equal(t, tc.expectFinal, tx.Status, "status %s should map to %s", tc.status, tc.expectFinal)
		})
	}
}
