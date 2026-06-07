package payment_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/infra/asaas"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
)

// ─── Simple counting mock provider ───────────────────────────────────────────

// countingProvider wraps the real MockProvider and counts CreatePayment calls.
// Used to verify the provider is called the correct number of times.
type countingProvider struct {
	inner           portout.PaymentProvider
	createCallCount int
	// forceError makes CreatePayment return an error on the Nth call.
	failOnCreate bool
}

func (c *countingProvider) CreateOrGetCustomer(ctx context.Context, userID, name, cpf, email string) (*portout.ProviderCustomer, error) {
	return c.inner.CreateOrGetCustomer(ctx, userID, name, cpf, email)
}

func (c *countingProvider) CreatePayment(ctx context.Context, req *portout.CreatePaymentRequest) (*portout.ProviderPayment, error) {
	c.createCallCount++
	if c.failOnCreate {
		return nil, fmt.Errorf("simulated provider failure")
	}
	return c.inner.CreatePayment(ctx, req)
}

func (c *countingProvider) GetPayment(ctx context.Context, providerID string) (*portout.ProviderPayment, error) {
	return c.inner.GetPayment(ctx, providerID)
}

func (c *countingProvider) GetPixQrCode(ctx context.Context, providerID string) (*portout.ProviderPixQrCode, error) {
	return c.inner.GetPixQrCode(ctx, providerID)
}

func (c *countingProvider) GetBoletoIdentificationField(ctx context.Context, providerID string) (*portout.ProviderIdentificationField, error) {
	return c.inner.GetBoletoIdentificationField(ctx, providerID)
}

func (c *countingProvider) TokenizeCreditCard(ctx context.Context, req *portout.TokenizeCreditCardRequest) (*portout.ProviderCardToken, error) {
	return c.inner.TokenizeCreditCard(ctx, req)
}

func (c *countingProvider) SimulateSandboxReceive(ctx context.Context, providerID string) error {
	return c.inner.SimulateSandboxReceive(ctx, providerID)
}

// confirmedProvider wraps MockProvider but returns ProviderStatusConfirmed from
// CreatePayment. Used to test the CC success path where installments are
// immediately marked PAID.
type confirmedProvider struct {
	portout.PaymentProvider
}

func (p *confirmedProvider) CreatePayment(ctx context.Context, req *portout.CreatePaymentRequest) (*portout.ProviderPayment, error) {
	pay, err := p.PaymentProvider.CreatePayment(ctx, req)
	if err != nil {
		return nil, err
	}
	pay.Status = portout.ProviderStatusConfirmed // simulate CC instant confirmation
	return pay, nil
}

// ─── Build helper ─────────────────────────────────────────────────────────────

// buildBatchIntegration wires ProcessBatchPayment with real MongoDB repos, the
// given provider, a mock fee policy, and a noop subledger.
//
// A pre-seeded customer link avoids the need for a real UsuarioRepository.
// The mockInstParticipanteRepo returns userID itself in FindParticipationIDsByUserID
// (configured per-test when participantID != userID) or an empty slice otherwise.
func buildBatchIntegration(
	t *testing.T,
	prov portout.PaymentProvider,
) (
	uc *ucpayment.ProcessBatchPayment,
	instRepo *mongodb.PaymentInstallmentRepository,
	txRepo *mongodb.PaymentTransactionRepository,
	linkRepo *mongodb.AsaasCustomerLinkRepository,
	feePolicy *mockFeePolicy,
	partRepo *mockInstParticipanteRepo,
) {
	t.Helper()

	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo = mongodb.NewPaymentTransactionRepository(client)
	instRepo = mongodb.NewPaymentInstallmentRepository(client)
	linkRepo = mongodb.NewAsaasCustomerLinkRepository(client)
	cardRepo := mongodb.NewSavedCreditCardRepository(client)

	feePolicy = new(mockFeePolicy)
	partRepo = new(mockInstParticipanteRepo)

	sub := new(mockSubledger)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	uc = ucpayment.NewProcessBatchPayment(
		instRepo, partRepo,
		txRepo, linkRepo,
		nil, // usuarioRepo: nil — customer link pre-seeded, won't be called
		cardRepo,
		prov,
		domain.PaymentProviderMock,
		feePolicy,
		sub,
	)
	return uc, instRepo, txRepo, linkRepo, feePolicy, partRepo
}

// seedInstallments inserts test installments into MongoDB and returns their IDs.
func seedInstallments(
	t *testing.T,
	repo *mongodb.PaymentInstallmentRepository,
	eventID, participantID string,
	statuses []domain.InstallmentStatus,
	amountCents int64,
) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, len(statuses))
	for i, st := range statuses {
		id := fmt.Sprintf("batch-inst-%s-%d", participantID, i)
		inst := &domain.PaymentInstallment{
			ID:                id,
			EventID:           eventID,
			ParticipantID:     participantID,
			InstallmentNumber: i + 1,
			TotalInstallments: len(statuses),
			AmountCents:       amountCents,
			DueDate:           time.Now().Add(30 * 24 * time.Hour),
			Status:            st,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, repo.Save(ctx, inst))
		ids[i] = id
	}
	return ids
}

// ─── AC Fase 2: Batch atomicity — provider failure ──────────────────────────

// TestProcessBatchPayment_Integration_ProviderFailure_ZeroInstallmentsAltered
// verifies that when the payment provider returns an error, ZERO installments
// are transitioned from PENDING — i.e. batch atomicity is preserved.
//
// Assertions (spec-169 §4 Fase 2 atomicidade):
//   - Response.Success == false.
//   - All installments remain in their original status (PENDING).
//   - Exactly one FAILED transaction document exists in MongoDB.
func TestProcessBatchPayment_Integration_ProviderFailure_ZeroInstallmentsAltered(t *testing.T) {
	failing := &countingProvider{
		inner:        asaas.NewMockProvider(),
		failOnCreate: true,
	}

	uc, instRepo, txRepo, linkRepo, feePolicy, partRepo := buildBatchIntegration(t, failing)
	ctx := context.Background()

	const (
		userID    = "user-batch-fail-1"
		eventID   = "evt-batch-fail-1"
		custID    = "cus_batch_fail"
		partID    = userID // simplest: participantID == userID
		amtCents  = int64(5000)
	)

	// Seed customer link and installments.
	seedCustomerLink(t, linkRepo, userID, custID)

	ids := seedInstallments(t, instRepo, eventID, partID,
		[]domain.InstallmentStatus{
			domain.InstallmentStatusPending,
			domain.InstallmentStatusPending,
			domain.InstallmentStatusOverdue,
		}, amtCents)

	feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
		Version: "v-batch-fail",
	}, nil)
	// userID in participationSet by default
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).Return([]string{}, nil)

	resp, err := uc.Execute(ctx, portin.ProcessBatchPaymentInput{
		UserID:         userID,
		InstallmentIDs: ids,
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	// Execute must not error — failure is encoded in the response.
	require.NoError(t, err)
	assert.False(t, resp.Success, "batch with provider failure must report failure")
	assert.Equal(t, 0, resp.ProcessedCount)
	assert.Contains(t, resp.Error, "simulated provider failure")

	// ── Verify installments are UNTOUCHED ────────────────────────────────────
	for _, id := range ids {
		inst, findErr := instRepo.FindByID(ctx, id)
		require.NoError(t, findErr)
		assert.NotEqual(t, domain.InstallmentStatusPaid, inst.Status,
			"installment %s must NOT be PAID after provider failure; got %s", id, inst.Status)
	}

	// ── Verify ONE failed transaction exists ─────────────────────────────────
	tx, findErr := txRepo.FindByID(ctx, resp.TransactionID)
	require.NoError(t, findErr)
	assert.Equal(t, domain.TransactionStatusFailed, tx.Status,
		"transaction must be FAILED after provider failure")
	assert.Contains(t, tx.FailureReason, "simulated provider failure")
	assert.Equal(t, 1, failing.createCallCount, "provider CreatePayment must be called exactly once")
}

// TestProcessBatchPayment_Integration_CreditCard_AllPaidAtomically verifies the
// CC success path: provider returns CONFIRMED → all installments are marked PAID
// in a single atomic MongoDB update.
//
// Assertions (spec-169 §4 Fase 2 atomicidade batch CC sucesso):
//   - Response.Success == true.
//   - ALL installments in the batch have status PAID.
//   - All PAID installments share the same TransactionID.
//   - The PaymentTransaction has status COMPLETED.
func TestProcessBatchPayment_Integration_CreditCard_AllPaidAtomically(t *testing.T) {
	ccConfirmed := &confirmedProvider{asaas.NewMockProvider()}

	uc, instRepo, txRepo, linkRepo, feePolicy, partRepo := buildBatchIntegration(t, ccConfirmed)
	ctx := context.Background()

	const (
		userID   = "user-batch-cc-1"
		eventID  = "evt-batch-cc-1"
		custID   = "cus_batch_cc"
		partID   = userID
		amtCents = int64(8000)
	)

	seedCustomerLink(t, linkRepo, userID, custID)

	ids := seedInstallments(t, instRepo, eventID, partID,
		[]domain.InstallmentStatus{
			domain.InstallmentStatusPending,
			domain.InstallmentStatusPending,
			domain.InstallmentStatusOverdue,
		}, amtCents)

	feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
		Version: "v-batch-cc",
	}, nil)
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).Return([]string{}, nil)

	resp, err := uc.Execute(ctx, portin.ProcessBatchPaymentInput{
		UserID:         userID,
		InstallmentIDs: ids,
		Method:         domain.PaymentMethodCreditCard,
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success, "CC batch with instant confirmation must succeed")
	assert.Equal(t, len(ids), resp.ProcessedCount)
	assert.Equal(t, int64(len(ids))*amtCents, resp.TotalAmountCents)

	// ── Verify ALL installments are PAID ─────────────────────────────────────
	for _, id := range ids {
		inst, findErr := instRepo.FindByID(ctx, id)
		require.NoError(t, findErr)
		assert.Equal(t, domain.InstallmentStatusPaid, inst.Status,
			"installment %s must be PAID after CC success", id)
		assert.Equal(t, resp.TransactionID, inst.TransactionID,
			"all installments must reference the same transaction")
	}

	// ── Verify the transaction is COMPLETED ──────────────────────────────────
	tx, findErr := txRepo.FindByID(ctx, resp.TransactionID)
	require.NoError(t, findErr)
	assert.Equal(t, domain.TransactionStatusCompleted, tx.Status,
		"transaction must be COMPLETED for CC with instant confirmation")
	assert.NotNil(t, tx.CompletedAt)
	assert.ElementsMatch(t, ids, tx.InstallmentIDs)
}

// TestProcessBatchPayment_Integration_PIX_InstallmentsRemainPending verifies
// that a PIX batch leaves installments in PENDING status — they remain pending
// until the webhook arrives (handled by HandlePaymentCallback).
//
// Assertions (spec-169 §4 Fase 2 PIX):
//   - Response.Success == true (charge created).
//   - Installments remain PENDING (webhook will PAID them).
//   - Transaction has status PENDING.
func TestProcessBatchPayment_Integration_PIX_InstallmentsRemainPending(t *testing.T) {
	uc, instRepo, txRepo, linkRepo, feePolicy, partRepo := buildBatchIntegration(t, asaas.NewMockProvider())
	ctx := context.Background()

	const (
		userID   = "user-batch-pix-1"
		eventID  = "evt-batch-pix-1"
		custID   = "cus_batch_pix"
		partID   = userID
		amtCents = int64(3000)
	)

	seedCustomerLink(t, linkRepo, userID, custID)

	ids := seedInstallments(t, instRepo, eventID, partID,
		[]domain.InstallmentStatus{
			domain.InstallmentStatusPending,
			domain.InstallmentStatusPending,
		}, amtCents)

	feePolicy.On("ResolveSnapshot", mock.Anything, eventID).Return(domain.FeePolicySnapshot{
		Version: "v-batch-pix",
	}, nil)
	partRepo.On("FindParticipationIDsByUserID", mock.Anything, userID).Return([]string{}, nil)

	resp, err := uc.Execute(ctx, portin.ProcessBatchPaymentInput{
		UserID:         userID,
		InstallmentIDs: ids,
		Method:         domain.PaymentMethodPix,
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Installments must remain PENDING for PIX.
	for _, id := range ids {
		inst, findErr := instRepo.FindByID(ctx, id)
		require.NoError(t, findErr)
		assert.Equal(t, domain.InstallmentStatusPending, inst.Status,
			"PIX installment %s must remain PENDING until webhook confirms", id)
	}

	// Transaction must be PENDING (provider confirmed PENDING for PIX).
	tx, findErr := txRepo.FindByID(ctx, resp.TransactionID)
	require.NoError(t, findErr)
	assert.Equal(t, domain.TransactionStatusPending, tx.Status)
}
