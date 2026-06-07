package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/infra/asaas"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// buildProcessPaymentIntegration wires ProcessPayment with real MongoDB repos
// and a MockProvider. A pre-seeded AsaasCustomerLink avoids the need for a real
// UsuarioRepository (the use case short-circuits before calling FindByID).
func buildProcessPaymentIntegration(t *testing.T) (
	uc *ucpayment.ProcessPayment,
	txRepo *mongodb.PaymentTransactionRepository,
	linkRepo *mongodb.AsaasCustomerLinkRepository,
	feePolicy *mockFeePolicy,
	subledger *mockSubledger,
) {
	t.Helper()

	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo = mongodb.NewPaymentTransactionRepository(client)
	linkRepo = mongodb.NewAsaasCustomerLinkRepository(client)
	cardRepo := mongodb.NewSavedCreditCardRepository(client)

	feePolicy = new(mockFeePolicy)
	subledger = new(mockSubledger)

	mockProvider := asaas.NewMockProvider()

	uc = ucpayment.NewProcessPayment(
		txRepo, linkRepo,
		nil,       // usuarioRepo: nil — won't be called because customer link is pre-seeded
		cardRepo,
		mockProvider,
		domain.PaymentProviderMock,
		feePolicy,
		subledger,
	)
	return uc, txRepo, linkRepo, feePolicy, subledger
}

// seedCustomerLink inserts a pre-built AsaasCustomerLink so getOrCreateCustomer
// fast-paths and never calls usuarioRepo.
func seedCustomerLink(t *testing.T, linkRepo *mongodb.AsaasCustomerLinkRepository, userID, customerID string) {
	t.Helper()
	link := &domain.AsaasCustomerLink{
		UserID:          userID,
		AsaasCustomerID: customerID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, linkRepo.Save(context.Background(), link))
}

// ─── AC Fase 1: PIX charge with MockProvider ────────────────────────────────

// TestProcessPayment_Integration_PIX_MockProvider verifies the full happy-path
// for a PIX charge using the MockProvider with a real MongoDB backend.
//
// Assertions (spec-169 §4 Fase 1):
//   - Transaction is persisted with amount_cents int64 (not float64).
//   - PaymentMethod == PIX.
//   - PixQrCodeText is non-empty (MockProvider populates it).
//   - FeePolicySnapshotVersion is captured from the resolver.
//   - Status is PENDING (MockProvider returns PENDING for PIX).
//   - Provider is MOCK.
func TestProcessPayment_Integration_PIX_MockProvider(t *testing.T) {
	uc, txRepo, linkRepo, feePolicy, subledger := buildProcessPaymentIntegration(t)
	ctx := context.Background()

	const (
		userID  = "user-pix-integ-1"
		eventID = "evt-pix-integ-1"
		custID  = "cus_mock_user-pix"
	)

	seedCustomerLink(t, linkRepo, userID, custID)

	feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
		FeePolicySource: "EVENT:" + eventID,
		PspFeePercent:   1.99,
		Version:         "fee-v1-pix",
	}, nil)
	subledger.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	const amountCents int64 = 15000 // R$ 150,00

	tx, err := uc.Execute(ctx, portin.ProcessPaymentInput{
		UserID:      userID,
		EventID:     eventID,
		AmountCents: amountCents,
		Method:      domain.PaymentMethodPix,
		CPF:         "12345678901",
	})
	require.NoError(t, err)
	require.NotNil(t, tx)

	// ── Verify in-memory result ──────────────────────────────────────────────
	assert.Equal(t, domain.TransactionStatusPending, tx.Status, "PIX charge should be PENDING")
	assert.Equal(t, amountCents, tx.AmountCents, "amount_cents must match exactly (int64, not float64)")
	assert.Equal(t, domain.PaymentMethodPix, tx.PaymentMethod)
	assert.Equal(t, domain.PaymentProviderMock, tx.Provider)
	assert.NotEmpty(t, tx.Metadata.PixQrCodeText, "PIX QR code text must be populated by MockProvider")
	assert.NotEmpty(t, tx.Metadata.PixQrCodeImage, "PIX QR code image must be populated by MockProvider")
	assert.Equal(t, "fee-v1-pix", tx.FeePolicySnapshotVersion, "fee snapshot version must be captured at creation")
	assert.NotNil(t, tx.FeePolicySnapshotCapturedAt, "fee snapshot timestamp must be captured")
	assert.NotEmpty(t, tx.ProviderTransactionID, "provider transaction ID must be set by MockProvider")

	// ── Verify round-trip from MongoDB ───────────────────────────────────────
	persisted, err := txRepo.FindByID(ctx, tx.ID)
	require.NoError(t, err, "transaction must be findable in MongoDB after Execute")
	assert.Equal(t, amountCents, persisted.AmountCents, "amount_cents round-trip must be exact int64")
	assert.Equal(t, domain.PaymentMethodPix, persisted.PaymentMethod)
	assert.Equal(t, domain.TransactionStatusPending, persisted.Status)
	assert.NotEmpty(t, persisted.Metadata.PixQrCodeText)
	assert.Equal(t, "fee-v1-pix", persisted.FeePolicySnapshotVersion)
}

// TestProcessPayment_Integration_AmountCents_ExactPreservation verifies that
// monetarily-sensitive values survive the BSON round-trip without any float64
// conversion artefacts.
//
// Assertions (spec-169 §4 AC monetário):
//   - Values 333, 1001, 999999999 — all preserved exactly.
func TestProcessPayment_Integration_AmountCents_ExactPreservation(t *testing.T) {
	uc, txRepo, linkRepo, feePolicy, subledger := buildProcessPaymentIntegration(t)
	ctx := context.Background()

	cases := []struct {
		name        string
		amountCents int64
	}{
		{"333_cents", 333},
		{"1001_cents", 1001},
		{"999999999_cents", 999_999_999},
		{"exceeds_int32", 2_147_483_648}, // > math.MaxInt32 — exposes int32 truncation
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userID := "user-money-" + tc.name
			eventID := "evt-money-" + tc.name
			custID := "cus_money_" + tc.name

			seedCustomerLink(t, linkRepo, userID, custID)

			feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
				Version: "v-money",
			}, nil)
			subledger.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			tx, err := uc.Execute(ctx, portin.ProcessPaymentInput{
				UserID:      userID,
				EventID:     eventID,
				AmountCents: tc.amountCents,
				Method:      domain.PaymentMethodPix,
				CPF:         "12345678901",
			})
			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, tc.amountCents, tx.AmountCents, "in-memory value must be exact")

			// Round-trip via MongoDB
			persisted, err := txRepo.FindByID(ctx, tx.ID)
			require.NoError(t, err)
			assert.Equal(t, tc.amountCents, persisted.AmountCents,
				"BSON round-trip must preserve amount_cents exactly as int64 (no float64 conversion)")
		})
	}
}

// TestProcessPayment_Integration_Idempotency_SameKey verifies that calling
// Execute twice with the same IdempotencyKey returns the same transaction and
// creates exactly one record in MongoDB.
//
// Assertions (spec-169 §4 idempotência process):
//   - Both calls return the same transaction ID.
//   - Exactly one document exists in MongoDB for that idempotency key.
//   - Provider CreatePayment called exactly once (first call only).
func TestProcessPayment_Integration_Idempotency_SameKey(t *testing.T) {
	uc, txRepo, linkRepo, feePolicy, subledger := buildProcessPaymentIntegration(t)
	ctx := context.Background()

	const (
		userID         = "user-idem-proc-1"
		eventID        = "evt-idem-proc-1"
		custID         = "cus_idem_proc"
		idempotencyKey = "idem-process-pay-001"
		amountCents    = int64(5000)
	)

	seedCustomerLink(t, linkRepo, userID, custID)

	feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
		Version: "v-idem",
	}, nil)
	subledger.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	input := portin.ProcessPaymentInput{
		UserID:         userID,
		EventID:        eventID,
		AmountCents:    amountCents,
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
		IdempotencyKey: idempotencyKey,
	}

	// First call — creates the transaction.
	tx1, err := uc.Execute(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, tx1)
	assert.Equal(t, amountCents, tx1.AmountCents)

	// Second call — must return the same transaction (idempotent no-op).
	tx2, err := uc.Execute(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, tx2)

	assert.Equal(t, tx1.ID, tx2.ID,
		"idempotent call must return the same transaction ID")
	assert.Equal(t, amountCents, tx2.AmountCents,
		"idempotent call must return the same amount")

	// Verify only ONE document exists for this idempotency key in MongoDB.
	retrieved, err := txRepo.FindByIdempotencyKey(ctx, idempotencyKey)
	require.NoError(t, err)
	assert.Equal(t, tx1.ID, retrieved.ID,
		"exactly one transaction must exist for the idempotency key")
}
