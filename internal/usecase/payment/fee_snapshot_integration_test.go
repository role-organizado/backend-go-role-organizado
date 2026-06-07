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

// TestFeeSnapshot_Integration_ImmutableAfterConfigChange verifies that the fee
// policy snapshot captured at transaction creation time is preserved even when
// the event's fee configuration is later changed.
//
// Assertions (spec-169 §4 Fase 4 fee snapshot):
//   - First charge captures fee v1 (PSP 2.0%, version "fee-v1").
//   - Config is then updated to fee v2 (PSP 3.5%, version "fee-v2").
//   - Second charge captures fee v2.
//   - First transaction still has FeePolicySnapshotVersion == "fee-v1".
//   - Retrieving the first transaction from MongoDB preserves the v1 snapshot.
func TestFeeSnapshot_Integration_ImmutableAfterConfigChange(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	// Set up real repositories backed by real MongoDB.
	txRepo := mongodb.NewPaymentTransactionRepository(client)
	linkRepo := mongodb.NewAsaasCustomerLinkRepository(client)
	cfgRepo := mongodb.NewConfigPagamentoRepository(client)
	cardRepo := mongodb.NewSavedCreditCardRepository(client)

	// Use the real FeePolicyService so it reads from evento_config_pagamentos.
	feePolicySvc := ucpayment.NewFeePolicyService(cfgRepo)

	// Noop subledger for all calls.
	sub := new(mockSubledger)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	prov := asaas.NewMockProvider()

	uc := ucpayment.NewProcessPayment(
		txRepo, linkRepo,
		nil, // usuarioRepo: nil — customer link pre-seeded
		cardRepo,
		prov,
		domain.PaymentProviderMock,
		feePolicySvc,
		sub,
	)

	const (
		userID  = "user-fee-snap-1"
		eventID = "evt-fee-snap-1"
		custID  = "cus_fee_snap"
	)

	// Pre-seed customer link.
	seedCustomerLink(t, linkRepo, userID, custID)

	// ── Step 1: Insert event config with fee v1 ──────────────────────────────
	cfgV1 := &domain.EventoConfigPagamento{
		EventoID:           eventID,
		PlatformFeePercent: 1.0,
		PspFeePercent:      2.0,
		FeePolicyVersion:   "fee-v1",
		CriadoEm:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	saved, err := cfgRepo.Save(ctx, cfgV1)
	require.NoError(t, err, "save fee config v1")

	// ── Step 2: Create first charge (should capture fee v1) ──────────────────
	tx1, err := uc.Execute(ctx, portin.ProcessPaymentInput{
		UserID:      userID,
		EventID:     eventID,
		AmountCents: 10000,
		Method:      domain.PaymentMethodPix,
		CPF:         "12345678901",
	})
	require.NoError(t, err)
	require.NotNil(t, tx1)
	assert.Equal(t, "fee-v1", tx1.FeePolicySnapshotVersion,
		"first transaction must capture fee v1 snapshot version")

	// ── Step 3: Update event config to fee v2 ────────────────────────────────
	saved.PspFeePercent = 3.5
	saved.PlatformFeePercent = 2.0
	saved.FeePolicyVersion = "fee-v2"
	saved.UpdatedAt = time.Now()
	_, err = cfgRepo.Update(ctx, saved)
	require.NoError(t, err, "update fee config to v2")

	// ── Step 4: Create second charge (should capture fee v2) ─────────────────
	// Need a different userID/custID to avoid reusing the old transaction state.
	const user2ID = "user-fee-snap-2"
	const cust2ID = "cus_fee_snap2"
	seedCustomerLink(t, linkRepo, user2ID, cust2ID)

	tx2, err := uc.Execute(ctx, portin.ProcessPaymentInput{
		UserID:      user2ID,
		EventID:     eventID,
		AmountCents: 5000,
		Method:      domain.PaymentMethodPix,
		CPF:         "98765432100",
	})
	require.NoError(t, err)
	require.NotNil(t, tx2)
	assert.Equal(t, "fee-v2", tx2.FeePolicySnapshotVersion,
		"second transaction must capture updated fee v2 snapshot version")

	// ── Step 5: Re-fetch first transaction and verify snapshot unchanged ──────
	persisted1, err := txRepo.FindByID(ctx, tx1.ID)
	require.NoError(t, err, "first transaction must still be findable")
	assert.Equal(t, "fee-v1", persisted1.FeePolicySnapshotVersion,
		"first transaction snapshot must be immutable — must still reflect fee-v1, not the updated fee-v2")
	assert.NotEqual(t, persisted1.FeePolicySnapshotVersion, tx2.FeePolicySnapshotVersion,
		"the two transactions must carry different snapshots")
}

// TestFeeSnapshot_Integration_GlobalFallback verifies that when no event-specific
// fee config exists, the GLOBAL policy is used (PSP 1.99%, version "global-v1").
func TestFeeSnapshot_Integration_GlobalFallback(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)
	linkRepo := mongodb.NewAsaasCustomerLinkRepository(client)
	cfgRepo := mongodb.NewConfigPagamentoRepository(client)
	cardRepo := mongodb.NewSavedCreditCardRepository(client)

	feePolicySvc := ucpayment.NewFeePolicyService(cfgRepo)

	sub := new(mockSubledger)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	uc := ucpayment.NewProcessPayment(
		txRepo, linkRepo, nil, cardRepo,
		asaas.NewMockProvider(),
		domain.PaymentProviderMock,
		feePolicySvc, sub,
	)

	const (
		userID  = "user-global-fee-1"
		eventID = "evt-no-config-999" // no config document for this event
		custID  = "cus_global_fee"
	)

	seedCustomerLink(t, linkRepo, userID, custID)

	tx, err := uc.Execute(ctx, portin.ProcessPaymentInput{
		UserID:      userID,
		EventID:     eventID,
		AmountCents: 20000,
		Method:      domain.PaymentMethodPix,
		CPF:         "12345678901",
	})
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Global fallback policy version is "global-v1".
	assert.Equal(t, "global-v1", tx.FeePolicySnapshotVersion,
		"when no event config exists, global policy (global-v1) must be used")
}
