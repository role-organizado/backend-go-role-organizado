package mongodb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixed UUIDs to avoid cross-test collisions inside the same container.
const (
	testFinanceSummaryEventUUID  = "aa0e8400-e29b-41d4-a716-446655440011"
	testFinanceSummaryEvent2UUID = "bb0e8400-e29b-41d4-a716-446655440012"
	testFinanceLedgerEventUUID   = "cc0e8400-e29b-41d4-a716-446655440013"
	testFinanceLedgerEvent2UUID  = "dd0e8400-e29b-41d4-a716-446655440014"
	testFinancePayUserUUID       = "ee0e8400-e29b-41d4-a716-446655440015"
	testFinancePayUser2UUID      = "ff0e8400-e29b-41d4-a716-446655440016"
)

// ─────────────────────────────────────────────────────────────────
// FinanceSummaryRepository tests
// ─────────────────────────────────────────────────────────────────

func TestFinanceSummary_SaveAndFindByEventID(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceSummaryRepository(client)
	ctx := context.Background()

	in := &domain.FinanceSummary{
		EventID:                testFinanceSummaryEventUUID,
		Goal:                   100_00,
		Collected:              50_00,
		ProgressPercentage:     50.0,
		AvailableForWithdrawal: 45_00,
		LastCalculatedAt:       time.Now().UTC().Truncate(time.Millisecond),
	}

	saved, err := repo.Save(ctx, in)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID, "Save must set a non-empty ID")

	found, err := repo.FindByEventID(ctx, testFinanceSummaryEventUUID)
	require.NoError(t, err)
	assert.Equal(t, testFinanceSummaryEventUUID, found.EventID)
	assert.Equal(t, int64(100_00), found.Goal)
	assert.Equal(t, int64(50_00), found.Collected)
	assert.Equal(t, float64(50.0), found.ProgressPercentage)
	assert.Equal(t, int64(45_00), found.AvailableForWithdrawal)
}

func TestFinanceSummary_FindByEventID_NotFound(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceSummaryRepository(client)
	ctx := context.Background()

	_, err := repo.FindByEventID(ctx, "00000000-0000-0000-0000-000000000099")
	require.Error(t, err)
}

func TestFinanceSummary_Update(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceSummaryRepository(client)
	ctx := context.Background()

	original := &domain.FinanceSummary{
		EventID:                testFinanceSummaryEvent2UUID,
		Goal:                   200_00,
		Collected:              0,
		ProgressPercentage:     0,
		AvailableForWithdrawal: 0,
	}
	saved, err := repo.Save(ctx, original)
	require.NoError(t, err)

	// Mutate and update
	saved.Collected = 120_00
	saved.ProgressPercentage = 60.0
	saved.AvailableForWithdrawal = 110_00

	updated, err := repo.Update(ctx, saved)
	require.NoError(t, err)
	assert.False(t, updated.LastCalculatedAt.IsZero(), "Update must set LastCalculatedAt")

	// Re-read and verify
	found, err := repo.FindByEventID(ctx, testFinanceSummaryEvent2UUID)
	require.NoError(t, err)
	assert.Equal(t, int64(120_00), found.Collected)
	assert.Equal(t, float64(60.0), found.ProgressPercentage)
	assert.Equal(t, int64(110_00), found.AvailableForWithdrawal)
}

// ─────────────────────────────────────────────────────────────────
// LedgerEntryRepository tests
// Uses direct collection inserts because LedgerEntryRepository is
// read-only (FindByEventID only — no Save in the port interface).
// ─────────────────────────────────────────────────────────────────

func insertLedgerEntries(t *testing.T, client *mongodb.Client, eventID, entryType string, count int, baseTime time.Time) {
	t.Helper()
	col := client.Collection("ledger_entries")
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := col.InsertOne(ctx, bson.D{
			{Key: "event_id", Value: mongodb.UUIDStringToBinary(eventID)},
			{Key: "type", Value: entryType},
			{Key: "amount", Value: int64(100 * (i + 1))},
			{Key: "description", Value: fmt.Sprintf("entry %d", i)},
			{Key: "occurred_at", Value: baseTime.Add(-time.Duration(i) * time.Hour)},
			{Key: "accounting_classification", Value: "REVENUE"},
		})
		require.NoError(t, err, "inserting test ledger entry %d", i)
	}
}

func TestFinanceLedger_Pagination(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewLedgerEntryRepository(client)
	ctx := context.Background()

	base := time.Now().UTC()
	insertLedgerEntries(t, client, testFinanceLedgerEventUUID, "CREDIT", 5, base)

	tests := []struct {
		name       string
		page, size int
		wantLen    int
		wantTotal  int64
	}{
		{"page1-size3", 1, 3, 3, 5},
		{"page2-size3", 2, 3, 2, 5},
		{"page3-size3", 3, 3, 0, 5},
		{"page1-size5", 1, 5, 5, 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries, total, err := repo.FindByEventID(ctx, testFinanceLedgerEventUUID, nil, nil, nil, tc.page, tc.size)
			require.NoError(t, err)
			assert.Equal(t, tc.wantTotal, total, "total count mismatch")
			assert.Len(t, entries, tc.wantLen, "page length mismatch")
		})
	}
}

func TestFinanceLedger_TypeFilter(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewLedgerEntryRepository(client)
	ctx := context.Background()

	base := time.Now().UTC()
	insertLedgerEntries(t, client, testFinanceLedgerEvent2UUID, "CREDIT", 3, base)
	insertLedgerEntries(t, client, testFinanceLedgerEvent2UUID, "DEBIT", 2, base)

	creditType := "CREDIT"
	entries, total, err := repo.FindByEventID(ctx, testFinanceLedgerEvent2UUID, &creditType, nil, nil, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, entries, 3)
	for _, e := range entries {
		assert.Equal(t, "CREDIT", e.Type)
	}
}

func TestFinanceLedger_SortedByOccurredAtDesc(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewLedgerEntryRepository(client)
	ctx := context.Background()

	// Use a unique event UUID for isolation
	eventID := "ee1e8400-e29b-41d4-a716-446655440017"
	base := time.Now().UTC().Truncate(time.Millisecond)
	insertLedgerEntries(t, client, eventID, "CREDIT", 3, base)

	entries, _, err := repo.FindByEventID(ctx, eventID, nil, nil, nil, 1, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// Entries should be newest first (occurred_at DESC)
	for i := 1; i < len(entries); i++ {
		assert.True(t,
			!entries[i].OccurredAt.After(entries[i-1].OccurredAt),
			"entry[%d].OccurredAt (%v) should be <= entry[%d].OccurredAt (%v)",
			i, entries[i].OccurredAt, i-1, entries[i-1].OccurredAt,
		)
	}
}

// ─────────────────────────────────────────────────────────────────
// PaymentAccountRepository tests
// ─────────────────────────────────────────────────────────────────

func TestFinancePaymentAccount_SaveAndFind(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceAccountRepository(client)
	ctx := context.Background()

	in := &domain.PaymentAccount{
		UserID:  testFinancePayUserUUID,
		Type:    "PIX",
		PixKey:  "test@example.com",
		PixType: "EMAIL",
	}
	saved, err := repo.Save(ctx, in)
	require.NoError(t, err)
	assert.NotEmpty(t, saved.ID)
	assert.True(t, saved.Active, "newly saved account must be active")

	// FindByID — happy path
	found, err := repo.FindByID(ctx, saved.ID, testFinancePayUserUUID)
	require.NoError(t, err)
	assert.Equal(t, testFinancePayUserUUID, found.UserID)
	assert.Equal(t, "PIX", found.Type)
	assert.Equal(t, "test@example.com", found.PixKey)

	// FindByID — wrong user must return NotFound
	_, err = repo.FindByID(ctx, saved.ID, "00000000-0000-0000-0000-000000000001")
	require.Error(t, err)

	// FindByUserID
	accounts, err := repo.FindByUserID(ctx, testFinancePayUserUUID)
	require.NoError(t, err)
	assert.Len(t, accounts, 1)
}

func TestFinancePaymentAccount_ClearDefault(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceAccountRepository(client)
	ctx := context.Background()

	// Insert 3 accounts — two with IsDefault=true
	for i := 0; i < 3; i++ {
		_, err := repo.Save(ctx, &domain.PaymentAccount{
			UserID:    testFinancePayUser2UUID,
			Type:      "PIX",
			PixKey:    fmt.Sprintf("key%d@test.com", i),
			PixType:   "EMAIL",
			IsDefault: i < 2,
		})
		require.NoError(t, err)
	}

	err := repo.ClearDefault(ctx, testFinancePayUser2UUID)
	require.NoError(t, err)

	accounts, err := repo.FindByUserID(ctx, testFinancePayUser2UUID)
	require.NoError(t, err)
	require.Len(t, accounts, 3)
	for _, a := range accounts {
		assert.False(t, a.IsDefault, "ClearDefault must unset is_default on all accounts")
	}
}

func TestFinancePaymentAccount_SoftDelete(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceAccountRepository(client)
	ctx := context.Background()

	// Use unique UUID to isolate from TestFinancePaymentAccount_ClearDefault
	userID := "a10e8400-e29b-41d4-a716-446655440018"

	// Save two accounts
	first, err := repo.Save(ctx, &domain.PaymentAccount{
		UserID:  userID,
		Type:    "PIX",
		PixKey:  "delete@test.com",
		PixType: "EMAIL",
	})
	require.NoError(t, err)

	_, err = repo.Save(ctx, &domain.PaymentAccount{
		UserID:  userID,
		Type:    "PIX",
		PixKey:  "keep@test.com",
		PixType: "EMAIL",
	})
	require.NoError(t, err)

	// Soft-delete the first account
	err = repo.SoftDelete(ctx, first.ID, userID)
	require.NoError(t, err)

	// FindByUserID must exclude the deleted account
	accounts, err := repo.FindByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, accounts, 1, "soft-deleted account must not appear in FindByUserID")
	assert.Equal(t, "keep@test.com", accounts[0].PixKey)

	// SoftDelete with wrong user must return NotFound
	err = repo.SoftDelete(ctx, first.ID, "00000000-0000-0000-0000-000000000002")
	require.Error(t, err)
}

func TestFinancePaymentAccount_SoftDelete_NotFound(t *testing.T) {
	client := testhelper.StartMongo(t)
	repo := mongodb.NewFinanceAccountRepository(client)
	ctx := context.Background()

	// Non-existent ObjectID hex
	err := repo.SoftDelete(ctx, "000000000000000000000099", testFinancePayUserUUID)
	require.Error(t, err)
}
