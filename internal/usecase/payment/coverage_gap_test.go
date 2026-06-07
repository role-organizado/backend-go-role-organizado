package payment_test

// coverage_gap_test.go — targeted unit tests to push internal/usecase/payment
// coverage from 61.3% to ≥70%.
//
// Covers:
//   - manage_saved_cards.go (all 0%)
//   - expire_transaction.go (all 0%)
//   - ProcessPayment.WithTemporalStarter (0%)
//   - ProcessPayment.Execute — Boleto path, CC tokenization path, CC instant-confirm
//   - ProcessPayment.startExpirationWorkflow (0%)
//   - HandlePaymentCallback.WithTemporalSignaler / WithTemporalStarter (both 0%)
//   - SubledgerDualWriteService (all 0%) via Testcontainers MongoDB

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authdomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Minimal Temporal mock adapters ────────────────────────────────────────────

type mockTemporalStarter struct{ mock.Mock }

func (m *mockTemporalStarter) StartWorkflow(ctx context.Context, opts portout.WorkflowStartOptions, fn interface{}, args ...interface{}) error {
	callArgs := m.Called(ctx, opts, fn)
	return callArgs.Error(0)
}

type mockTemporalSignaler struct{ mock.Mock }

func (m *mockTemporalSignaler) SignalWorkflow(ctx context.Context, workflowID, signal string, arg interface{}) error {
	callArgs := m.Called(ctx, workflowID, signal, arg)
	return callArgs.Error(0)
}

// ─── manage_saved_cards.go coverage ────────────────────────────────────────────

func TestManageSavedCards_List_Success(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	cards := []*domain.SavedCreditCard{
		{ID: "card-1", UserID: "u1", LastFourDigits: "4242", IsActive: true},
	}
	repo.On("FindByUserID", mock.Anything, "u1").Return(cards, nil)

	got, err := uc.List(context.Background(), "u1")
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, "card-1", got[0].ID)
	repo.AssertExpectations(t)
}

func TestManageSavedCards_List_Error(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	repo.On("FindByUserID", mock.Anything, "u2").Return(nil, errors.New("db error"))

	got, err := uc.List(context.Background(), "u2")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "db error")
}

func TestManageSavedCards_SetDefault_Success(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	repo.On("SetDefault", mock.Anything, "u1", "card-1").Return(nil)

	err := uc.SetDefault(context.Background(), "card-1", "u1")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestManageSavedCards_SetDefault_Error(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	repo.On("SetDefault", mock.Anything, "u1", "card-x").Return(errors.New("not found"))

	err := uc.SetDefault(context.Background(), "card-x", "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManageSavedCards_Delete_Success(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	repo.On("DeleteByID", mock.Anything, "card-del").Return(nil)

	err := uc.Delete(context.Background(), "card-del", "u1")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestManageSavedCards_Delete_Error(t *testing.T) {
	repo := new(mockSavedCardRepo)
	uc := ucpayment.NewManageSavedCards(repo)

	repo.On("DeleteByID", mock.Anything, "card-missing").Return(errors.New("delete failed"))

	err := uc.Delete(context.Background(), "card-missing", "u1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete failed")
}

// ─── expire_transaction.go coverage ────────────────────────────────────────────

func TestExpireTransaction_NotFound_NoOp(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewExpireTransaction(txRepo)

	txRepo.On("FindByID", mock.Anything, "tx-gone").
		Return(nil, apierr.NotFound("transaction", "tx-gone"))

	err := uc.Execute(context.Background(), "tx-gone")
	require.NoError(t, err, "not-found must be treated as a no-op (idempotent)")
	txRepo.AssertNotCalled(t, "Update")
}

func TestExpireTransaction_FindError(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewExpireTransaction(txRepo)

	txRepo.On("FindByID", mock.Anything, "tx-err").
		Return(nil, errors.New("mongo timeout"))

	err := uc.Execute(context.Background(), "tx-err")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mongo timeout")
}

func TestExpireTransaction_AlreadyTerminal_ReturnsSpecialError(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewExpireTransaction(txRepo)

	// COMPLETED is terminal — expiration is a no-op but signals the caller.
	txRepo.On("FindByID", mock.Anything, "tx-done").Return(&domain.PaymentTransaction{
		ID:     "tx-done",
		Status: domain.TransactionStatusCompleted,
	}, nil)

	err := uc.Execute(context.Background(), "tx-done")
	// The use case returns a sentinel error for already-terminal transactions.
	require.Error(t, err, "expected non-nil error for terminal transaction")
	txRepo.AssertNotCalled(t, "Update")
}

func TestExpireTransaction_PendingMarkedCancelledTimeout(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewExpireTransaction(txRepo)

	tx := &domain.PaymentTransaction{
		ID:     "tx-pending",
		Status: domain.TransactionStatusPending,
	}
	txRepo.On("FindByID", mock.Anything, "tx-pending").Return(tx, nil)
	txRepo.On("Update", mock.Anything, mock.MatchedBy(func(updated *domain.PaymentTransaction) bool {
		return updated.Status == domain.TransactionStatusCancelled &&
			updated.FailureReason == "TIMEOUT"
	})).Return(nil)

	err := uc.Execute(context.Background(), "tx-pending")
	require.NoError(t, err)
	txRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
}

// ─── ProcessPayment.WithTemporalStarter ─────────────────────────────────────────

func TestProcessPayment_WithTemporalStarter(t *testing.T) {
	uc, _, _, _, _, _, _, _ := buildUseCase(t)
	starter := new(mockTemporalStarter)
	// WithTemporalStarter returns *ProcessPayment (fluent), verify it doesn't panic.
	got := uc.WithTemporalStarter(starter)
	require.NotNil(t, got, "WithTemporalStarter must return non-nil *ProcessPayment")
}

// ─── HandlePaymentCallback.WithTemporalSignaler / WithTemporalStarter ─────────

func TestHandlePaymentCallback_WithTemporalSetters(t *testing.T) {
	txRepo := new(mockTxRepo)
	instRepo := new(mockInstallmentRepo)
	webhookRepo := new(mockWebhookRepo)

	uc := ucpayment.NewHandlePaymentCallback(txRepo, instRepo, webhookRepo, nil, nil, nil)

	starter := new(mockTemporalStarter)
	signaler := new(mockTemporalSignaler)

	// Both setters must return *HandlePaymentCallback for fluent chaining.
	ucWithSignaler := uc.WithTemporalSignaler(signaler)
	require.NotNil(t, ucWithSignaler)

	ucWithStarter := uc.WithTemporalStarter(starter)
	require.NotNil(t, ucWithStarter)
}

// ─── ProcessPayment.Execute — Boleto path ──────────────────────────────────────

func TestProcessPayment_BoletoFlow(t *testing.T) {
	uc, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-boleto-1").
		Return(nil, apierr.NotFound("tx", "idem-boleto-1"))

	// Customer: cache miss → create.
	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(nil, apierr.NotFound("link", "user-1"))
	usuRepo.On("FindByID", mock.Anything, "user-1").Return(&authdomain.Usuario{
		ID: "user-1", Nome: "User Boleto", Email: "boleto@test.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, "user-1", mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "cust-boleto"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-boleto").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	dueDate := time.Now().Add(3 * 24 * time.Hour)
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:          "asaas-boleto-1",
		Status:      portout.ProviderStatusPending,
		BankSlipUrl: "https://boleto.url/123",
		DueDate:     dueDate.Format("2006-01-02"),
	}, nil)

	// Boleto identification field.
	prov.On("GetBoletoIdentificationField", mock.Anything, "asaas-boleto-1").
		Return(&portout.ProviderIdentificationField{
			IdentificationField: "123450000019 1 00001234560007 7 00001234560001 8 60000100000",
			NossoNumero:         "00001234560001",
		}, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-boleto",
		AmountCents:    20000,
		Method:         domain.PaymentMethodBoleto,
		IdempotencyKey: "idem-boleto-1",
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.TransactionStatusPending, got.Status)
	assert.Equal(t, domain.PaymentMethodBoleto, got.PaymentMethod)
	assert.NotEmpty(t, got.Metadata.BoletoDigitableLine, "boleto line must be populated")
	assert.NotEmpty(t, got.Metadata.BoletoCode, "boleto nosso numero must be populated")
	assert.NotNil(t, got.ExpiresAt, "boleto expiry must be set from due date")
}

// ─── ProcessPayment.Execute — CC with tokenization ─────────────────────────────

func TestProcessPayment_CreditCard_Tokenization_Success(t *testing.T) {
	uc, txRepo, linkRepo, usuRepo, cardRepo, prov, fee, sub := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-cc-tok").
		Return(nil, apierr.NotFound("tx", "idem-cc-tok"))

	// Cached customer to simplify.
	linkRepo.On("FindByUserID", mock.Anything, "user-cc").Return(&domain.AsaasCustomerLink{
		UserID:          "user-cc",
		AsaasCustomerID: "cust-cc",
	}, nil)
	// usuRepo not called (customer cached)
	_ = usuRepo

	fee.On("ResolveSnapshot", mock.Anything, "evt-cc").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// TokenizeCreditCard must be called once.
	prov.On("TokenizeCreditCard", mock.Anything, mock.MatchedBy(func(req *portout.TokenizeCreditCardRequest) bool {
		return req.Number == "4111111111111111"
	})).Return(&portout.ProviderCardToken{
		Token:    "tok_cc_123",
		LastFour: "1111",
		Brand:    "VISA",
	}, nil)

	// CC confirmed immediately.
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-cc-1",
		Status: portout.ProviderStatusConfirmed,
	}, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	// Save card path: SaveCard=true.
	cardRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.SavedCreditCard")).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-cc",
		EventID:        "evt-cc",
		AmountCents:    30000,
		Method:         domain.PaymentMethodCreditCard,
		IdempotencyKey: "idem-cc-tok",
		CPF:            "12345678901",
		SaveCard:       true,
		CreditCard: &portin.CreditCardInput{
			HolderName:  "Test User",
			Number:      "4111111111111111",
			ExpiryMonth: "12",
			ExpiryYear:  "2030",
			CVV:         "123",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	// CC with CONFIRMED status → MarkCompleted.
	assert.Equal(t, domain.TransactionStatusCompleted, got.Status,
		"CC with ProviderStatusConfirmed must be marked COMPLETED immediately")
	assert.Equal(t, "1111", got.Metadata.CardLast4)
	// Verify tokenize was called.
	prov.AssertCalled(t, "TokenizeCreditCard", mock.Anything, mock.Anything)
	// Verify card was saved.
	cardRepo.AssertCalled(t, "Save", mock.Anything, mock.Anything)
}

// ─── ProcessPayment.Execute — CC with tokenization failure ─────────────────────

func TestProcessPayment_CreditCard_TokenizationFailure_MarksTransactionFailed(t *testing.T) {
	uc, txRepo, linkRepo, _, _, prov, fee, sub := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-cc-tokfail").
		Return(nil, apierr.NotFound("tx", "idem-cc-tokfail"))

	linkRepo.On("FindByUserID", mock.Anything, "user-cctok").Return(&domain.AsaasCustomerLink{
		UserID:          "user-cctok",
		AsaasCustomerID: "cust-cctok",
	}, nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-cctok").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	prov.On("TokenizeCreditCard", mock.Anything, mock.Anything).
		Return(nil, errors.New("card declined"))
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-cctok",
		EventID:        "evt-cctok",
		AmountCents:    10000,
		Method:         domain.PaymentMethodCreditCard,
		IdempotencyKey: "idem-cc-tokfail",
		CPF:            "12345678901",
		CreditCard: &portin.CreditCardInput{
			HolderName:  "Bad Card",
			Number:      "4000000000000002",
			ExpiryMonth: "01",
			ExpiryYear:  "2025",
			CVV:         "999",
		},
	})

	require.NoError(t, err, "tokenization failure encodes into transaction status, not error return")
	require.NotNil(t, got)
	assert.Equal(t, domain.TransactionStatusFailed, got.Status,
		"CC tokenization failure must set transaction to FAILED")
	assert.Contains(t, got.FailureReason, "card tokenisation failed")
	// CreatePayment must NOT be called after tokenization failure.
	prov.AssertNotCalled(t, "CreatePayment")
}

// ─── ProcessPayment.startExpirationWorkflow ─────────────────────────────────────
//
// Verifies that when WithTemporalStarter is set and the PIX QR code returns an
// ExpiresAt, the use case calls temporalStarter.StartWorkflow exactly once.

func TestProcessPayment_PIX_WithTemporalStarter_StartsExpirationWorkflow(t *testing.T) {
	uc, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildUseCase(t)

	starter := new(mockTemporalStarter)
	uc.WithTemporalStarter(starter)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-pix-temporal").
		Return(nil, apierr.NotFound("tx", "idem-pix-temporal"))

	// Customer: cache miss → create.
	linkRepo.On("FindByUserID", mock.Anything, "user-temporal").
		Return(nil, apierr.NotFound("link", "user-temporal"))
	usuRepo.On("FindByID", mock.Anything, "user-temporal").Return(&authdomain.Usuario{
		ID: "user-temporal", Nome: "Temporal User", Email: "temporal@test.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, "user-temporal", mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "cust-temporal"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-temporal").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-temporal-1",
		Status: portout.ProviderStatusPending,
	}, nil)

	// PIX QR code with explicit expiry — this sets tx.ExpiresAt, triggering startExpirationWorkflow.
	expiresAt := time.Now().Add(30 * time.Minute)
	prov.On("GetPixQrCode", mock.Anything, "asaas-temporal-1").Return(&portout.ProviderPixQrCode{
		EncodedImage:   "qr-base64",
		Payload:        "00020126pix-payload",
		ExpirationDate: expiresAt,
	}, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	// Temporal starter must be invoked.
	starter.On("StartWorkflow", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-temporal",
		EventID:        "evt-temporal",
		AmountCents:    5000,
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-pix-temporal",
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, domain.TransactionStatusPending, got.Status)
	// Verify temporal starter was called once (for the expiration workflow).
	starter.AssertCalled(t, "StartWorkflow", mock.Anything, mock.Anything, mock.Anything)
}

// ─── SubledgerDualWriteService integration tests ────────────────────────────────
//
// Uses Testcontainers MongoDB to exercise the real SubledgerDualWriteService
// against live collections. Covers NewSubledgerDualWriteService,
// AppendPaymentCommitment (happy path + idempotent duplicate), and
// AppendPaymentConfirmation.

func TestSubledger_Integration_AppendPaymentCommitment(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()

	ledgerEntries := client.Collection("ledger_entries")
	ledgerSnapshot := client.Collection("ledger_snapshot_events")

	svc := ucpayment.NewSubledgerDualWriteService(ledgerEntries, ledgerSnapshot)
	require.NotNil(t, svc)

	now := time.Now()
	tx := &domain.PaymentTransaction{
		ID:             "sub-tx-1",
		UserID:         "user-sub-1",
		EventID:        "evt-sub-1",
		AmountCents:    10000,
		PaymentMethod:  domain.PaymentMethodPix,
		Provider:       domain.PaymentProviderMock,
		IdempotencyKey: "idem-sub-1",
		CreatedAt:      now,
	}
	fees := domain.CalculateFees(tx.AmountCents, domain.FeePolicySnapshot{
		PspFeePercent:      1.99,
		PlatformFeePercent: 1.0,
	})
	snapshot := domain.FeePolicySnapshot{
		Version:        "v-sub-1",
		FeePolicySource: "GLOBAL",
		PspFeePercent:  1.99,
	}

	// First call — must succeed.
	err := svc.AppendPaymentCommitment(ctx, tx, fees, snapshot)
	require.NoError(t, err, "first AppendPaymentCommitment must succeed")

	// Second call with same idempotency key — must be a silent no-op (idempotent).
	err = svc.AppendPaymentCommitment(ctx, tx, fees, snapshot)
	require.NoError(t, err, "duplicate AppendPaymentCommitment must be treated as no-op")
}

func TestSubledger_Integration_AppendPaymentConfirmation(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()

	ledgerEntries := client.Collection("ledger_entries")
	ledgerSnapshot := client.Collection("ledger_snapshot_events")

	svc := ucpayment.NewSubledgerDualWriteService(ledgerEntries, ledgerSnapshot)

	now := time.Now()
	completedAt := now
	tx := &domain.PaymentTransaction{
		ID:                    "sub-tx-confirm-1",
		UserID:                "user-sub-2",
		EventID:               "evt-sub-2",
		AmountCents:           20000,
		PaymentMethod:         domain.PaymentMethodCreditCard,
		Provider:              domain.PaymentProviderMock,
		ProviderTransactionID: "prov-sub-2",
		Status:                domain.TransactionStatusCompleted,
		CompletedAt:           &completedAt,
		CreatedAt:             now,
	}

	// First confirmation — must succeed.
	err := svc.AppendPaymentConfirmation(ctx, tx)
	require.NoError(t, err, "first AppendPaymentConfirmation must succeed")

	// Duplicate confirmation — must be silent no-op.
	err = svc.AppendPaymentConfirmation(ctx, tx)
	require.NoError(t, err, "duplicate AppendPaymentConfirmation must be no-op (idempotent)")
}
