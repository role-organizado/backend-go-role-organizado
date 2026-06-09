package mongodb_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────────────────────
// PaymentTransactionRepository
// ──────────────────────────────────────────────────────────────

func TestPaymentTransactionRepository_SaveAndFindByID(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentTransactionRepository(client)

	tx := &domain.PaymentTransaction{
		ID:            "tx-roundtrip-1",
		UserID:        "user-1",
		AmountCents:   150000,
		Currency:      "BRL",
		PaymentMethod: domain.PaymentMethodPix,
		Provider:      domain.PaymentProviderAsaas,
		Status:        domain.TransactionStatusPending,
		CreatedAt:     time.Now().Truncate(time.Millisecond),
		UpdatedAt:     time.Now().Truncate(time.Millisecond),
	}

	require.NoError(t, repo.Save(ctx, tx))

	found, err := repo.FindByID(ctx, tx.ID)
	require.NoError(t, err)
	assert.Equal(t, tx.ID, found.ID)
	assert.Equal(t, int64(150000), found.AmountCents, "round-trip preserves amount_cents int64")
	assert.Equal(t, domain.PaymentMethodPix, found.PaymentMethod)
	assert.Equal(t, domain.PaymentProviderAsaas, found.Provider)
}

func TestPaymentTransactionRepository_IdempotencyKey_DuplicateReturnExisting(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentTransactionRepository(client)

	// First save — succeeds normally.
	tx1 := &domain.PaymentTransaction{
		ID:             "tx-idem-1",
		UserID:         "user-2",
		IdempotencyKey: "idem-key-001",
		AmountCents:    5000,
		PaymentMethod:  domain.PaymentMethodBoleto,
		Provider:       domain.PaymentProviderAsaas,
		Status:         domain.TransactionStatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.Save(ctx, tx1))

	// Second save with same idempotency key but different ID/amount — must return
	// the first transaction's data (idempotency behaviour mirroring Java).
	tx2 := &domain.PaymentTransaction{
		ID:             "tx-idem-2",
		UserID:         "user-2",
		IdempotencyKey: "idem-key-001",
		AmountCents:    9999, // different amount
		PaymentMethod:  domain.PaymentMethodPix,
		Provider:       domain.PaymentProviderAsaas,
		Status:         domain.TransactionStatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.Save(ctx, tx2), "second save with duplicate idempotencyKey must not error")
	assert.Equal(t, "tx-idem-1", tx2.ID, "caller's struct is overwritten with the existing transaction ID")
	assert.Equal(t, int64(5000), tx2.AmountCents, "amount_cents matches the original transaction")
}

func TestPaymentTransactionRepository_FindPendingOlderThan(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentTransactionRepository(client)

	old := &domain.PaymentTransaction{
		ID:            "tx-old-1",
		UserID:        "user-3",
		AmountCents:   100,
		PaymentMethod: domain.PaymentMethodPix,
		Provider:      domain.PaymentProviderAsaas,
		Status:        domain.TransactionStatusPending,
		CreatedAt:     time.Now().Add(-48 * time.Hour),
		UpdatedAt:     time.Now().Add(-48 * time.Hour),
	}
	recent := &domain.PaymentTransaction{
		ID:            "tx-recent-1",
		UserID:        "user-3",
		AmountCents:   200,
		PaymentMethod: domain.PaymentMethodPix,
		Provider:      domain.PaymentProviderAsaas,
		Status:        domain.TransactionStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	require.NoError(t, repo.Save(ctx, old))
	require.NoError(t, repo.Save(ctx, recent))

	threshold := time.Now().Add(-24 * time.Hour)
	results, err := repo.FindPendingOlderThan(ctx, threshold)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "tx-old-1", results[0].ID)
}

// ──────────────────────────────────────────────────────────────
// PaymentInstallmentRepository
// ──────────────────────────────────────────────────────────────

func TestPaymentInstallmentRepository_MarkPaidBatch_Atomic(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentInstallmentRepository(client)

	ids := []string{"inst-1", "inst-2", "inst-3"}
	for i, id := range ids {
		inst := &domain.PaymentInstallment{
			ID:                id,
			EventID:           "evt-1",
			ParticipantID:     "part-1",
			InstallmentNumber: i + 1,
			TotalInstallments: 3,
			AmountCents:       10000,
			DueDate:           time.Now().Add(30 * 24 * time.Hour),
			Status:            domain.InstallmentStatusPending,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, repo.Save(ctx, inst))
	}

	paidAt := time.Now()
	require.NoError(t, repo.MarkPaidBatch(ctx, ids, "tx-batch-1", paidAt, "PIX", "ref-001"))

	for _, id := range ids {
		found, err := repo.FindByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, domain.InstallmentStatusPaid, found.Status)
		assert.Equal(t, "tx-batch-1", found.TransactionID)
		assert.Equal(t, "PIX", found.PaymentMethod)
		assert.Equal(t, "ref-001", found.PaymentReference)
	}
}

func TestPaymentInstallmentRepository_CancelByParticipant_OnlyPendingAndOverdue(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentInstallmentRepository(client)

	statuses := []domain.InstallmentStatus{
		domain.InstallmentStatusPending,
		domain.InstallmentStatusOverdue,
		domain.InstallmentStatusPaid,     // must NOT be cancelled
		domain.InstallmentStatusCancelled, // already done
	}
	for i, st := range statuses {
		inst := &domain.PaymentInstallment{
			ID:            fmt.Sprintf("cancel-inst-%d", i),
			EventID:       "evt-cancel",
			ParticipantID: "part-cancel",
			AmountCents:   5000,
			DueDate:       time.Now(),
			Status:        st,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		require.NoError(t, repo.Save(ctx, inst))
	}

	n, err := repo.CancelByParticipant(ctx, "evt-cancel", "part-cancel")
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "only PENDING and OVERDUE should be cancelled")

	paid, err := repo.FindByID(ctx, "cancel-inst-2")
	require.NoError(t, err)
	assert.Equal(t, domain.InstallmentStatusPaid, paid.Status, "PAID must remain PAID")
}

// ──────────────────────────────────────────────────────────────
// PaymentAccountRepository
// ──────────────────────────────────────────────────────────────

func TestPaymentAccountRepository_SetDefault_UnmarksOthers(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentAccountRepository(client)

	acct1 := &domain.PaymentAccount{
		ID:          "acct-1",
		UserID:      "user-setdefault",
		AccountType: domain.AccountTypePix,
		IsDefault:   false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	acct2 := &domain.PaymentAccount{
		ID:          "acct-2",
		UserID:      "user-setdefault",
		AccountType: domain.AccountTypePix,
		IsDefault:   false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	require.NoError(t, repo.Save(ctx, acct1))
	require.NoError(t, repo.Save(ctx, acct2))

	// Set acct1 as default.
	require.NoError(t, repo.SetDefault(ctx, "user-setdefault", "acct-1"))
	a1, err := repo.FindByID(ctx, "acct-1")
	require.NoError(t, err)
	assert.True(t, a1.IsDefault, "acct-1 should be default")

	// Now switch default to acct2.
	require.NoError(t, repo.SetDefault(ctx, "user-setdefault", "acct-2"))

	a1After, err := repo.FindByID(ctx, "acct-1")
	require.NoError(t, err)
	assert.False(t, a1After.IsDefault, "acct-1 must be unmarked after SetDefault(acct-2)")

	a2After, err := repo.FindByID(ctx, "acct-2")
	require.NoError(t, err)
	assert.True(t, a2After.IsDefault, "acct-2 must be the new default")
}

func TestPaymentAccountRepository_DeleteByID_SoftDelete(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()

	repo := mongodb.NewPaymentAccountRepository(client)

	acct := &domain.PaymentAccount{
		ID:        "acct-soft-1",
		UserID:    "user-soft",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, repo.Save(ctx, acct))
	require.NoError(t, repo.DeleteByID(ctx, "acct-soft-1"))

	// Direct lookup still finds the document but it is inactive.
	found, err := repo.FindByID(ctx, "acct-soft-1")
	require.NoError(t, err)
	assert.False(t, found.IsActive, "soft-deleted account must have is_active=false")

	// FindByUserID should NOT return the inactive account.
	all, err := repo.FindByUserID(ctx, "user-soft")
	require.NoError(t, err)
	assert.Empty(t, all, "inactive accounts must be excluded from FindByUserID")
}

// ──────────────────────────────────────────────────────────────
// SavedCreditCardRepository
// ──────────────────────────────────────────────────────────────

func TestSavedCreditCardRepository_SetDefault_UnmarksOthers(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewSavedCreditCardRepository(client)

	card1 := &domain.SavedCreditCard{
		ID:             "card-1",
		UserID:         "user-cards",
		LastFourDigits: "1234",
		Brand:          domain.CardBrandVisa,
		IsDefault:      false,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	card2 := &domain.SavedCreditCard{
		ID:             "card-2",
		UserID:         "user-cards",
		LastFourDigits: "5678",
		Brand:          domain.CardBrandMastercard,
		IsDefault:      false,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	require.NoError(t, repo.Save(ctx, card1))
	require.NoError(t, repo.Save(ctx, card2))

	require.NoError(t, repo.SetDefault(ctx, "user-cards", "card-1"))
	require.NoError(t, repo.SetDefault(ctx, "user-cards", "card-2"))

	c1, err := repo.FindByID(ctx, "card-1")
	require.NoError(t, err)
	assert.False(t, c1.IsDefault, "card-1 must be unmarked")

	c2, err := repo.FindByID(ctx, "card-2")
	require.NoError(t, err)
	assert.True(t, c2.IsDefault, "card-2 must be the new default")
}

// ──────────────────────────────────────────────────────────────
// ProcessedWebhookEventRepository
// ──────────────────────────────────────────────────────────────

func TestProcessedWebhookEventRepository_SaveUnique_IdempotencyOnDuplicate(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	// Index required for duplicate-key enforcement.
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewProcessedWebhookEventRepository(client)

	evt := &domain.ProcessedWebhookEvent{
		Provider:              "ASAAS",
		EventID:               "asaas-webhook-001",
		ProviderTransactionID: "pay_abc123",
		EventType:             "PAYMENT_RECEIVED",
		ProcessedAt:           time.Now(),
	}

	// First call succeeds.
	require.NoError(t, repo.SaveUnique(ctx, evt))

	// Second call with same (provider, eventId) must return ErrAlreadyProcessed.
	evt2 := &domain.ProcessedWebhookEvent{
		Provider:    "ASAAS",
		EventID:     "asaas-webhook-001",
		ProcessedAt: time.Now(),
	}
	err := repo.SaveUnique(ctx, evt2)
	assert.True(t, errors.Is(err, out.ErrAlreadyProcessed), "second call must return ErrAlreadyProcessed, got: %v", err)

	// ExistsByProviderAndEventID must return true.
	exists, err := repo.ExistsByProviderAndEventID(ctx, "ASAAS", "asaas-webhook-001")
	require.NoError(t, err)
	assert.True(t, exists)
}

// ──────────────────────────────────────────────────────────────
// AsaasCustomerLinkRepository
// ──────────────────────────────────────────────────────────────

func TestAsaasCustomerLinkRepository_SaveAndFind(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()

	repo := mongodb.NewAsaasCustomerLinkRepository(client)

	link := &domain.AsaasCustomerLink{
		UserID:          "user-asaas-1",
		AsaasCustomerID: "cus_asaas123",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, repo.Save(ctx, link))

	found, err := repo.FindByUserID(ctx, "user-asaas-1")
	require.NoError(t, err)
	assert.Equal(t, "user-asaas-1", found.UserID)
	assert.Equal(t, "cus_asaas123", found.AsaasCustomerID)
}

func TestAsaasCustomerLinkRepository_Upsert_IdempotentSave(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()

	repo := mongodb.NewAsaasCustomerLinkRepository(client)

	link := &domain.AsaasCustomerLink{
		UserID:          "user-asaas-upsert",
		AsaasCustomerID: "cus_first",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, repo.Save(ctx, link))

	// Save again (upsert) with updated customer ID.
	link.AsaasCustomerID = "cus_second"
	require.NoError(t, repo.Save(ctx, link), "upsert must not fail on duplicate _id")

	found, err := repo.FindByUserID(ctx, "user-asaas-upsert")
	require.NoError(t, err)
	assert.Equal(t, "cus_second", found.AsaasCustomerID)
}

// ──────────────────────────────────────────────────────────────
// Round-trip: domain → BSON → domain preserves amount_cents int64
// ──────────────────────────────────────────────────────────────

func TestPaymentTransactionRepository_AmountCents_RoundTrip(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	repo := mongodb.NewPaymentTransactionRepository(client)

	cases := []struct {
		name        string
		amountCents int64
	}{
		{"zero", 0},
		{"one_cent", 1},
		{"large", 999_999_999},
		{"max_int32_plus", 2_147_483_648}, // exceeds int32 max — must survive BSON round-trip as int64
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &domain.PaymentTransaction{
				ID:            "tx-cents-" + tc.name,
				UserID:        "user-cents",
				AmountCents:   tc.amountCents,
				PaymentMethod: domain.PaymentMethodPix,
				Provider:      domain.PaymentProviderMock,
				Status:        domain.TransactionStatusPending,
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}
			require.NoError(t, repo.Save(ctx, tx))

			found, err := repo.FindByID(ctx, tx.ID)
			require.NoError(t, err)
			assert.Equal(t, tc.amountCents, found.AmountCents, "amount_cents must survive BSON round-trip")
		})
	}
}
