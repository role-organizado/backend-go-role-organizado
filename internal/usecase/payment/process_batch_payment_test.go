package payment_test

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
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildBatchUseCase wires a ProcessBatchPayment with all mock dependencies.
// Returns the use case plus each mock for per-test expectation setup.
func buildBatchUseCase(t *testing.T) (
	*ucpayment.ProcessBatchPayment,
	*mockInstallmentRepo,
	*mockInstParticipanteRepo,
	*mockTxRepo,
	*mockCustomerLinkRepo,
	*mockUsuarioRepo,
	*mockSavedCardRepo,
	*mockProvider,
	*mockFeePolicy,
	*mockSubledger,
) {
	t.Helper()

	instRepo := new(mockInstallmentRepo)
	partRepo := new(mockInstParticipanteRepo)
	txRepo := new(mockTxRepo)
	linkRepo := new(mockCustomerLinkRepo)
	usuRepo := new(mockUsuarioRepo)
	cardRepo := new(mockSavedCardRepo)
	prov := new(mockProvider)
	fee := new(mockFeePolicy)
	sub := new(mockSubledger)

	uc := ucpayment.NewProcessBatchPayment(
		instRepo, partRepo,
		txRepo, linkRepo, usuRepo, cardRepo,
		prov, domain.PaymentProviderMock,
		fee, sub,
	)
	return uc, instRepo, partRepo, txRepo, linkRepo, usuRepo, cardRepo, prov, fee, sub
}

// sampleInstallment builds a PaymentInstallment owned by the given participantID.
func sampleInstallment(id, eventID, participantID string, amountCents int64, status domain.InstallmentStatus) *domain.PaymentInstallment {
	return &domain.PaymentInstallment{
		ID:            id,
		EventID:       eventID,
		ParticipantID: participantID,
		AmountCents:   amountCents,
		Status:        status,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// stubBatchCustomerAndFee sets up the most common customer + fee mocks.
func stubBatchCustomerAndFee(
	txRepo *mockTxRepo,
	linkRepo *mockCustomerLinkRepo,
	usuRepo *mockUsuarioRepo,
	prov *mockProvider,
	fee *mockFeePolicy,
	sub *mockSubledger,
	userID, eventID string,
) {
	linkRepo.On("FindByUserID", mock.Anything, userID).Return(nil, apierr.NotFound("link", userID))
	usuRepo.On("FindByID", mock.Anything, userID).Return(&authdomain.Usuario{
		ID: userID, Nome: "Test User", Email: "test@test.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, userID, mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "asaas-cust-batch"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	fee.On("ResolveSnapshot", mock.Anything, eventID).Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

// TestProcessBatch_HappyCreditCard: two installments paid via CC → both PAID,
// single transaction created, success=true.
func TestProcessBatch_HappyCreditCard(t *testing.T) {
	uc, instRepo, partRepo, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildBatchUseCase(t)

	inst1 := sampleInstallment("inst-1", "evt-1", "part-user-1", 5000, domain.InstallmentStatusPending)
	inst2 := sampleInstallment("inst-2", "evt-1", "part-user-1", 3000, domain.InstallmentStatusOverdue)

	instRepo.On("FindByIDs", mock.Anything, []string{"inst-1", "inst-2"}).
		Return([]*domain.PaymentInstallment{inst1, inst2}, nil)

	// Ownership: participantIDs include "part-user-1".
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-user-1"}, nil)

	// Idempotency miss (no existing tx for this key).
	txRepo.On("FindByIdempotencyKey", mock.Anything, "batch-cc-idem").
		Return(nil, apierr.NotFound("tx", "batch-cc-idem"))

	stubBatchCustomerAndFee(txRepo, linkRepo, usuRepo, prov, fee, sub, "user-1", "evt-1")

	// Provider: CC charge, immediately confirmed.
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-batch-cc",
		Status: portout.ProviderStatusConfirmed,
	}, nil)

	// MarkPaidBatch must be called with both installment IDs.
	instRepo.On("MarkPaidBatch",
		mock.Anything,
		[]string{"inst-1", "inst-2"},
		mock.AnythingOfType("string"), // txID (uuid)
		mock.AnythingOfType("time.Time"),
		"CREDIT_CARD",
		"asaas-batch-cc",
	).Return(nil)

	resp, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-1", "inst-2"},
		Method:         domain.PaymentMethodCreditCard,
		IdempotencyKey: "batch-cc-idem",
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 2, resp.ProcessedCount)
	assert.Equal(t, int64(8000), resp.TotalAmountCents) // 5000 + 3000
	assert.NotEmpty(t, resp.TransactionID)
	assert.Empty(t, resp.Error)

	instRepo.AssertCalled(t, "MarkPaidBatch", mock.Anything, []string{"inst-1", "inst-2"},
		mock.Anything, mock.Anything, "CREDIT_CARD", "asaas-batch-cc")
	txRepo.AssertCalled(t, "Save", mock.Anything, mock.Anything)
}

// TestProcessBatch_OwnershipViolation: one installment belongs to another user → 403,
// no installments altered, no transaction created.
func TestProcessBatch_OwnershipViolation(t *testing.T) {
	uc, instRepo, partRepo, txRepo, _, _, _, _, _, _ := buildBatchUseCase(t)

	myInst := sampleInstallment("inst-mine", "evt-1", "part-user-1", 5000, domain.InstallmentStatusPending)
	otherInst := sampleInstallment("inst-other", "evt-1", "part-user-2", 3000, domain.InstallmentStatusPending)

	instRepo.On("FindByIDs", mock.Anything, []string{"inst-mine", "inst-other"}).
		Return([]*domain.PaymentInstallment{myInst, otherInst}, nil)

	// User "user-1" only has participationID "part-user-1".
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-user-1"}, nil)

	_, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-mine", "inst-other"},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae), "expected APIError, got %T: %v", err, ae)
	assert.Equal(t, 403, ae.Status)

	// Transaction must NOT be created.
	txRepo.AssertNotCalled(t, "Save")
	instRepo.AssertNotCalled(t, "MarkPaidBatch")
}

// TestProcessBatch_AlreadyPaidInstallment: one installment is PAID → 400,
// no transaction created.
func TestProcessBatch_AlreadyPaidInstallment(t *testing.T) {
	uc, instRepo, partRepo, txRepo, _, _, _, _, _, _ := buildBatchUseCase(t)

	pendingInst := sampleInstallment("inst-pend", "evt-1", "part-u1", 5000, domain.InstallmentStatusPending)
	paidInst := sampleInstallment("inst-paid", "evt-1", "part-u1", 3000, domain.InstallmentStatusPaid)

	instRepo.On("FindByIDs", mock.Anything, []string{"inst-pend", "inst-paid"}).
		Return([]*domain.PaymentInstallment{pendingInst, paidInst}, nil)

	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-u1"}, nil)

	_, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-pend", "inst-paid"},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae), "expected APIError, got %T: %v", err, ae)
	assert.Equal(t, 400, ae.Status)

	txRepo.AssertNotCalled(t, "Save")
}

// TestProcessBatch_ProviderFailure: provider returns error → transaction FAILED,
// no installments altered (MarkPaidBatch never called).
func TestProcessBatch_ProviderFailure(t *testing.T) {
	uc, instRepo, partRepo, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildBatchUseCase(t)

	inst := sampleInstallment("inst-1", "evt-2", "part-u1", 2000, domain.InstallmentStatusPending)
	instRepo.On("FindByIDs", mock.Anything, []string{"inst-1"}).
		Return([]*domain.PaymentInstallment{inst}, nil)
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-u1"}, nil)

	stubBatchCustomerAndFee(txRepo, linkRepo, usuRepo, prov, fee, sub, "user-1", "evt-2")

	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(nil, errors.New("psp down"))

	resp, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-1"},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	// Failure is encoded in the response; Execute must not return an error.
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "psp down")
	assert.Equal(t, 0, resp.ProcessedCount)

	// Installments must NOT be altered.
	instRepo.AssertNotCalled(t, "MarkPaidBatch")
}

// TestProcessBatch_CentSumCorrect: verifies the total is the arithmetic sum of
// all installment AmountCents, not a rounded or float value.
func TestProcessBatch_CentSumCorrect(t *testing.T) {
	uc, instRepo, partRepo, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildBatchUseCase(t)

	// Three installments with amounts that would expose float rounding issues.
	amounts := []int64{333, 334, 333} // sum = 1000
	ids := []string{"inst-a", "inst-b", "inst-c"}
	insts := make([]*domain.PaymentInstallment, len(ids))
	for i, id := range ids {
		insts[i] = sampleInstallment(id, "evt-3", "part-u1", amounts[i], domain.InstallmentStatusPending)
	}

	instRepo.On("FindByIDs", mock.Anything, ids).Return(insts, nil)
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-u1"}, nil)

	// Reset and re-stub for evt-3
	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(nil, apierr.NotFound("link", "user-1"))
	usuRepo.On("FindByID", mock.Anything, "user-1").Return(&authdomain.Usuario{
		ID: "user-1", Nome: "T", Email: "t@t.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, "user-1", mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "cust-sum"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	fee.On("ResolveSnapshot", mock.Anything, "evt-3").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-sum-pix",
		Status: portout.ProviderStatusPending,
	}, nil)
	prov.On("GetPixQrCode", mock.Anything, "asaas-sum-pix").Return(&portout.ProviderPixQrCode{
		Payload:        "pix-payload",
		ExpirationDate: time.Now().Add(30 * time.Minute),
	}, nil)

	resp, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: ids,
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1000), resp.TotalAmountCents) // 333+334+333
}

// TestProcessBatch_PixStaysInstallmentsPending: PIX batch → transaction PENDING,
// installments NOT marked PAID (webhook will do it in T007).
func TestProcessBatch_PixStaysInstallmentsPending(t *testing.T) {
	uc, instRepo, partRepo, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildBatchUseCase(t)

	inst := sampleInstallment("inst-pix", "evt-4", "part-u1", 5000, domain.InstallmentStatusPending)
	instRepo.On("FindByIDs", mock.Anything, []string{"inst-pix"}).
		Return([]*domain.PaymentInstallment{inst}, nil)
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, "user-1").
		Return([]string{"part-u1"}, nil)

	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(nil, apierr.NotFound("link", "user-1"))
	usuRepo.On("FindByID", mock.Anything, "user-1").Return(&authdomain.Usuario{
		ID: "user-1", Nome: "Pix User", Email: "pix@test.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, "user-1", mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "cust-pix-batch"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	fee.On("ResolveSnapshot", mock.Anything, "evt-4").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-pix-batch",
		Status: portout.ProviderStatusPending,
	}, nil)
	prov.On("GetPixQrCode", mock.Anything, "asaas-pix-batch").Return(&portout.ProviderPixQrCode{
		EncodedImage:   "img",
		Payload:        "00020126",
		ExpirationDate: time.Now().Add(30 * time.Minute),
	}, nil)

	resp, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-pix"},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, 1, resp.ProcessedCount)

	// MarkPaidBatch must NOT be called for PIX (async webhook path).
	instRepo.AssertNotCalled(t, "MarkPaidBatch")

	// The transaction must have been created and updated (still PENDING).
	txRepo.AssertCalled(t, "Save", mock.Anything, mock.Anything)
	txRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
}

// TestProcessBatch_EmptyIDs: empty installmentIds → 400 before any DB call.
func TestProcessBatch_EmptyIDs(t *testing.T) {
	uc, instRepo, _, txRepo, _, _, _, _, _, _ := buildBatchUseCase(t)

	_, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, 400, ae.Status)

	instRepo.AssertNotCalled(t, "FindByIDs")
	txRepo.AssertNotCalled(t, "Save")
}

// TestProcessBatch_MissingInstallment: one ID not found → 404.
func TestProcessBatch_MissingInstallment(t *testing.T) {
	uc, instRepo, _, txRepo, _, _, _, _, _, _ := buildBatchUseCase(t)

	// Only "inst-found" is returned; "inst-missing" is absent.
	found := sampleInstallment("inst-found", "evt-1", "part-u1", 1000, domain.InstallmentStatusPending)
	instRepo.On("FindByIDs", mock.Anything, []string{"inst-found", "inst-missing"}).
		Return([]*domain.PaymentInstallment{found}, nil)

	_, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-found", "inst-missing"},
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, 404, ae.Status)

	txRepo.AssertNotCalled(t, "Save")
}

// TestProcessBatch_Idempotency: same idempotencyKey → returns existing transaction
// without calling the provider again.
func TestProcessBatch_Idempotency(t *testing.T) {
	uc, instRepo, _, txRepo, _, _, _, prov, _, _ := buildBatchUseCase(t)

	existing := &domain.PaymentTransaction{
		ID:             "tx-existing-batch",
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-1", "inst-2"},
		AmountCents:    8000,
		Status:         domain.TransactionStatusCompleted,
	}
	txRepo.On("FindByIdempotencyKey", mock.Anything, "batch-idem-1").Return(existing, nil)

	resp, err := uc.Execute(context.Background(), portin.ProcessBatchPaymentInput{
		UserID:         "user-1",
		InstallmentIDs: []string{"inst-1", "inst-2"},
		Method:         domain.PaymentMethodCreditCard,
		IdempotencyKey: "batch-idem-1",
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, "tx-existing-batch", resp.TransactionID)
	assert.Equal(t, 2, resp.ProcessedCount)
	assert.Equal(t, int64(8000), resp.TotalAmountCents)

	// Provider must NOT be called.
	prov.AssertNotCalled(t, "CreatePayment")
	instRepo.AssertNotCalled(t, "FindByIDs")
}
