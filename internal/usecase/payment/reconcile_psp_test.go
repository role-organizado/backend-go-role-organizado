package payment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
)

// ─── Mock: PaymentProvider (reused from process_payment_test.go in same package) ──

// Note: mockTxRepo and mockProvider are already defined in process_payment_test.go
// (same package payment_test). We reuse them directly here.

// ─── Helpers ─────────────────────────────────────────────────────────────────

func pendingOlderThan(id, providerID string) *domain.PaymentTransaction {
	return &domain.PaymentTransaction{
		ID:                    id,
		UserID:                "user-1",
		EventID:               "evt-1",
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: providerID,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderAsaas,
		AmountCents:           10000,
		CreatedAt:             time.Now().Add(-30 * time.Minute), // older than threshold
		UpdatedAt:             time.Now().Add(-30 * time.Minute),
	}
}

func buildReconcileUC(txRepo *mockTxRepo, prov *mockProvider) *ucpayment.ReconcilePspTransactions {
	return ucpayment.NewReconcilePspTransactions(txRepo, prov)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestReconcile_DetectsDivergencePendingToReceived verifies that when a local transaction
// is PENDING but the provider reports RECEIVED, the use case updates the status to COMPLETED.
func TestReconcile_DetectsDivergencePendingToReceived(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	tx := pendingOlderThan("tx-recon-1", "prov-123")

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{tx}, nil)

	prov.On("GetPayment", mock.Anything, "prov-123").
		Return(&portout.ProviderPayment{
			ID:     "prov-123",
			Status: portout.ProviderStatusReceived, // divergence: provider says RECEIVED
		}, nil)

	txRepo.On("Update", mock.Anything, mock.MatchedBy(func(tx *domain.PaymentTransaction) bool {
		return tx.Status == domain.TransactionStatusCompleted
	})).Return(nil)

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{
		From: time.Now().Add(-24 * time.Hour),
		To:   time.Now(),
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(1), result.Updated)
	assert.Equal(t, int64(0), result.Failed)

	txRepo.AssertExpectations(t)
	prov.AssertExpectations(t)
}

// TestReconcile_NoActionWhenProviderStillPending verifies that when the provider also
// reports PENDING, no update is performed.
func TestReconcile_NoActionWhenProviderStillPending(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	tx := pendingOlderThan("tx-recon-2", "prov-456")

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{tx}, nil)

	prov.On("GetPayment", mock.Anything, "prov-456").
		Return(&portout.ProviderPayment{
			ID:     "prov-456",
			Status: portout.ProviderStatusPending, // provider also still PENDING
		}, nil)

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{
		From: time.Now().Add(-24 * time.Hour),
		To:   time.Now(),
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(0), result.Updated) // no update
	assert.Equal(t, int64(0), result.Failed)

	txRepo.AssertNotCalled(t, "Update")
	txRepo.AssertExpectations(t)
	prov.AssertExpectations(t)
}

// TestReconcile_DetectsCancel verifies that when provider reports CANCELED,
// the local transaction is updated to CANCELLED.
func TestReconcile_DetectsCancel(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	tx := pendingOlderThan("tx-recon-3", "prov-789")

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{tx}, nil)

	prov.On("GetPayment", mock.Anything, "prov-789").
		Return(&portout.ProviderPayment{
			ID:     "prov-789",
			Status: portout.ProviderStatusCanceled,
		}, nil)

	txRepo.On("Update", mock.Anything, mock.MatchedBy(func(tx *domain.PaymentTransaction) bool {
		return tx.Status == domain.TransactionStatusCancelled
	})).Return(nil)

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{})

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(1), result.Updated)

	txRepo.AssertExpectations(t)
	prov.AssertExpectations(t)
}

// TestReconcile_SkipsTransactionWithoutProviderID verifies that transactions without
// a ProviderTransactionID are skipped (no GetPayment call).
func TestReconcile_SkipsTransactionWithoutProviderID(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	tx := pendingOlderThan("tx-recon-4", "") // no provider ID
	tx.ProviderTransactionID = ""

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{tx}, nil)

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{})

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(0), result.Updated)
	assert.Equal(t, int64(0), result.Failed)

	prov.AssertNotCalled(t, "GetPayment")
	txRepo.AssertExpectations(t)
}

// TestReconcile_ProviderError verifies that a provider GetPayment error increments
// Failed without aborting the reconciliation pass.
func TestReconcile_ProviderError(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	tx := pendingOlderThan("tx-recon-5", "prov-err")

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{tx}, nil)

	prov.On("GetPayment", mock.Anything, "prov-err").
		Return(nil, errors.New("provider unavailable"))

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{})

	require.NoError(t, err) // reconcile itself doesn't fail
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(0), result.Updated)
	assert.Equal(t, int64(1), result.Failed) // error counted

	txRepo.AssertNotCalled(t, "Update")
	txRepo.AssertExpectations(t)
	prov.AssertExpectations(t)
}

// TestReconcile_EmptyResult verifies that when no pending transactions are found,
// the result has zeroed counts with no error.
func TestReconcile_EmptyResult(t *testing.T) {
	txRepo := new(mockTxRepo)
	prov := new(mockProvider)

	txRepo.On("FindPendingOlderThan", mock.Anything, mock.AnythingOfType("time.Time")).
		Return([]*domain.PaymentTransaction{}, nil)

	uc := buildReconcileUC(txRepo, prov)
	result, err := uc.Execute(context.Background(), portin.ReconcileFilter{})

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Checked)
	assert.Equal(t, int64(0), result.Updated)
	assert.Equal(t, int64(0), result.Failed)
}
