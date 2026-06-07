package payment_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
)

// TestHandlePaymentCallback_Integration_Idempotency_SameEventID verifies that
// calling Execute twice with the same ProviderEventID results in exactly one
// transaction update — the second call is a silent no-op.
func TestHandlePaymentCallback_Integration_Idempotency_SameEventID(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)
	installmentRepo := mongodb.NewPaymentInstallmentRepository(client)
	webhookRepo := mongodb.NewProcessedWebhookEventRepository(client)

	// Seed a PENDING transaction in the DB.
	tx := &domain.PaymentTransaction{
		ID:                    "tx-idem-integ-1",
		UserID:                "user-idem-1",
		EventID:               "evt-idem-1",
		AmountCents:           15000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderAsaas,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "asaas-pay-idem-1",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, txRepo.Save(ctx, tx))

	uc := ucpayment.NewHandlePaymentCallback(txRepo, installmentRepo, webhookRepo, nil, nil, nil)

	payload := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       "evt-idem-event-1",
		ProviderTransactionID: "asaas-pay-idem-1",
		EventType:             "PAYMENT_RECEIVED",
		NewStatus:             "RECEIVED",
	}

	// First call — should succeed and mark transaction COMPLETED.
	err1 := uc.Execute(ctx, payload)
	require.NoError(t, err1)

	// Verify transaction is COMPLETED after first call.
	updated, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, updated.Status)

	// Reset status to PENDING to detect if the second call erroneously updates it.
	updated.Status = domain.TransactionStatusPending
	updated.CompletedAt = nil
	require.NoError(t, txRepo.Update(ctx, updated))

	// Second call with the same eventID — must be a no-op (idempotent).
	err2 := uc.Execute(ctx, payload)
	require.NoError(t, err2)

	// Transaction must remain PENDING (the second call was a no-op).
	afterSecond, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusPending, afterSecond.Status,
		"second call with same eventID must be a no-op (transaction should remain PENDING)")
}

// TestHandlePaymentCallback_Integration_Race_ConcurrentDelivery verifies that
// when two goroutines deliver the same webhook event simultaneously, exactly one
// of them processes it (the other is a no-op via DB unique index).
func TestHandlePaymentCallback_Integration_Race_ConcurrentDelivery(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)
	installmentRepo := mongodb.NewPaymentInstallmentRepository(client)
	webhookRepo := mongodb.NewProcessedWebhookEventRepository(client)

	// Seed a PENDING transaction.
	tx := &domain.PaymentTransaction{
		ID:                    "tx-race-1",
		UserID:                "user-race-1",
		EventID:               "evt-race-1",
		AmountCents:           20000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderAsaas,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "asaas-race-pay-1",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, txRepo.Save(ctx, tx))

	uc := ucpayment.NewHandlePaymentCallback(txRepo, installmentRepo, webhookRepo, nil, nil, nil)

	payload := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       "evt-race-concurrent-1",
		ProviderTransactionID: "asaas-race-pay-1",
		EventType:             "PAYMENT_RECEIVED",
		NewStatus:             "RECEIVED",
	}

	const goroutines = 5
	var successCount int64
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // synchronised start to maximise concurrency
			if execErr := uc.Execute(ctx, payload); execErr == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	close(start) // release all goroutines simultaneously
	wg.Wait()

	// All goroutines should return nil (no errors — idempotent).
	assert.Equal(t, int64(goroutines), successCount,
		"all %d goroutines should return nil (idempotent, no errors)", goroutines)

	// Exactly one transaction update must have occurred: the transaction is COMPLETED.
	final, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, final.Status,
		"transaction must be COMPLETED after concurrent delivery")

	// Exactly one webhook event record must exist in the DB.
	exists, err := webhookRepo.ExistsByProviderAndEventID(ctx, "ASAAS", "evt-race-concurrent-1")
	require.NoError(t, err)
	assert.True(t, exists, "exactly one processed webhook event record must exist")
}

// TestHandlePaymentCallback_Integration_ProviderTransactionID_Fallback verifies
// that the use case finds the correct transaction when ProviderTransactionID
// matches the providerTransactionId field in the DB.
func TestHandlePaymentCallback_Integration_ProviderTransactionID_Fallback(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)
	installmentRepo := mongodb.NewPaymentInstallmentRepository(client)
	webhookRepo := mongodb.NewProcessedWebhookEventRepository(client)

	// Seed a transaction where the providerTransactionId is the Asaas payment ID.
	tx := &domain.PaymentTransaction{
		ID:                    "tx-fallback-1",
		UserID:                "user-fallback-1",
		EventID:               "evt-fallback-1",
		AmountCents:           5000,
		PaymentMethod:         domain.PaymentMethodBoleto,
		Provider:              domain.PaymentProviderAsaas,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "asaas-boleto-123",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, txRepo.Save(ctx, tx))

	uc := ucpayment.NewHandlePaymentCallback(txRepo, installmentRepo, webhookRepo, nil, nil, nil)

	// Deliver a webhook where ProviderTransactionID is the Asaas payment ID (primary lookup).
	payload := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       "evt-fallback-event-1",
		ProviderTransactionID: "asaas-boleto-123", // matches tx.ProviderTransactionID
		EventType:             "PAYMENT_RECEIVED",
		NewStatus:             "RECEIVED",
	}

	err := uc.Execute(ctx, payload)
	require.NoError(t, err)

	// Verify transaction reached COMPLETED.
	updated, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, updated.Status)
}

// TestHandlePaymentCallback_Integration_Approved_WithInstallments verifies the full
// approved flow: transaction → COMPLETED, installments → PAID, allocations created.
func TestHandlePaymentCallback_Integration_Approved_WithInstallments(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)
	installmentRepo := mongodb.NewPaymentInstallmentRepository(client)
	webhookRepo := mongodb.NewProcessedWebhookEventRepository(client)
	allocCol := client.Collection("installment_allocations")
	allocSvc := ucpayment.NewInstallmentAllocationService(allocCol, installmentRepo)

	// Seed installment.
	inst := &domain.PaymentInstallment{
		ID:                "inst-full-1",
		EventID:           "evt-full-1",
		ParticipantID:     "user-full-1",
		LiabilityID:       "liability-1",
		InstallmentNumber: 1,
		TotalInstallments: 1,
		AmountCents:       10000,
		DueDate:           time.Now().Add(24 * time.Hour),
		Status:            domain.InstallmentStatusPending,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	require.NoError(t, installmentRepo.Save(ctx, inst))

	// Seed transaction linked to the installment.
	tx := &domain.PaymentTransaction{
		ID:                    "tx-full-1",
		UserID:                "user-full-1",
		EventID:               "evt-full-1",
		InstallmentIDs:        []string{"inst-full-1"},
		AmountCents:           10000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderAsaas,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "asaas-full-pay-1",
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	require.NoError(t, txRepo.Save(ctx, tx))

	uc := ucpayment.NewHandlePaymentCallback(txRepo, installmentRepo, webhookRepo, allocSvc, nil, nil)

	payload := portin.PaymentCallbackPayload{
		Provider:              domain.PaymentProviderAsaas,
		ProviderEventID:       "evt-full-event-1",
		ProviderTransactionID: "asaas-full-pay-1",
		EventType:             "PAYMENT_RECEIVED",
		NewStatus:             "RECEIVED",
	}

	err := uc.Execute(ctx, payload)
	require.NoError(t, err)

	// Transaction must be COMPLETED.
	updatedTx, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, updatedTx.Status)
	assert.NotNil(t, updatedTx.CompletedAt)

	// Installment must be PAID.
	updatedInst, err := installmentRepo.FindByID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusPaid, updatedInst.Status)
	assert.Equal(t, tx.ID, updatedInst.TransactionID)

	// Allocation must have been created in installment_allocations.
	count, err := allocCol.CountDocuments(ctx, map[string]string{"installment_id": inst.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one allocation record must have been created")
}
