package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	domain_event "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/internal/testhelper"
	"github.com/role-organizado/backend-go-role-organizado/migrations"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Reconcile PSP integration tests ─────────────────────────────────────────

// testReconcileProvider is a configurable mock provider for reconcile tests.
// GetPayment returns different statuses depending on the providerID queried.
type testReconcileProvider struct {
	// paymentStatuses maps providerTransactionID → status to return from GetPayment
	paymentStatuses map[string]portout.PaymentProviderStatus
}

func (p *testReconcileProvider) CreateOrGetCustomer(_ context.Context, _, _, _, _ string) (*portout.ProviderCustomer, error) {
	return nil, apierr.Internal("not implemented in testReconcileProvider")
}

func (p *testReconcileProvider) CreatePayment(_ context.Context, _ *portout.CreatePaymentRequest) (*portout.ProviderPayment, error) {
	return nil, apierr.Internal("not implemented in testReconcileProvider")
}

func (p *testReconcileProvider) GetPayment(_ context.Context, providerID string) (*portout.ProviderPayment, error) {
	if st, ok := p.paymentStatuses[providerID]; ok {
		return &portout.ProviderPayment{ID: providerID, Status: st}, nil
	}
	return &portout.ProviderPayment{ID: providerID, Status: portout.ProviderStatusPending}, nil
}

func (p *testReconcileProvider) GetPixQrCode(_ context.Context, _ string) (*portout.ProviderPixQrCode, error) {
	return nil, apierr.Internal("not implemented in testReconcileProvider")
}

func (p *testReconcileProvider) GetBoletoIdentificationField(_ context.Context, _ string) (*portout.ProviderIdentificationField, error) {
	return nil, apierr.Internal("not implemented in testReconcileProvider")
}

func (p *testReconcileProvider) TokenizeCreditCard(_ context.Context, _ *portout.TokenizeCreditCardRequest) (*portout.ProviderCardToken, error) {
	return nil, apierr.Internal("not implemented in testReconcileProvider")
}

func (p *testReconcileProvider) SimulateSandboxReceive(_ context.Context, _ string) error {
	return nil
}

// TestReconcilePsp_Integration_CorrectsPendingDivergence verifies that the
// reconcile use case finds a PENDING transaction older than the threshold and
// updates it to COMPLETED when the provider reports RECEIVED.
//
// Assertions (spec-169 §4 Fase 4 reconcile):
//   - Checked == 1.
//   - Updated == 1.
//   - The transaction in MongoDB has status COMPLETED after reconcile.
func TestReconcilePsp_Integration_CorrectsPendingDivergence(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)

	// Seed a PENDING transaction created 48h ago (older than any reasonable threshold).
	oldTx := &domain.PaymentTransaction{
		ID:                    "tx-reconcile-1",
		UserID:                "user-reconcile-1",
		EventID:               "evt-reconcile-1",
		AmountCents:           12000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderMock,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "prov-reconcile-1",
		CreatedAt:             time.Now().Add(-48 * time.Hour),
		UpdatedAt:             time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, txRepo.Save(ctx, oldTx))

	// The provider says this charge was RECEIVED.
	prov := &testReconcileProvider{
		paymentStatuses: map[string]portout.PaymentProviderStatus{
			"prov-reconcile-1": portout.ProviderStatusReceived,
		},
	}

	uc := ucpayment.NewReconcilePspTransactions(txRepo, prov)

	// Threshold: 24h ago — oldTx (48h old) is within the scan window.
	threshold := time.Now().Add(-24 * time.Hour)
	result, err := uc.Execute(ctx, portin.ReconcileFilter{From: threshold})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked, "one transaction must be checked")
	assert.Equal(t, int64(1), result.Updated, "one transaction must be updated")
	assert.Equal(t, int64(0), result.Failed)

	// Verify the transaction is now COMPLETED in MongoDB.
	updated, err := txRepo.FindByID(ctx, oldTx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusCompleted, updated.Status,
		"reconcile must correct PENDING→COMPLETED when provider reports RECEIVED")
	assert.NotNil(t, updated.CompletedAt, "CompletedAt must be set after reconcile")
}

// TestReconcilePsp_Integration_RecentTransactionNotTouched verifies that
// transactions created within the threshold window are NOT reconciled.
func TestReconcilePsp_Integration_RecentTransactionNotTouched(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)

	// Seed a PENDING transaction created just 1 minute ago (very recent).
	recentTx := &domain.PaymentTransaction{
		ID:                    "tx-recent-reconcile-1",
		UserID:                "user-reconcile-recent",
		AmountCents:           5000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderMock,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "prov-recent-1",
		CreatedAt:             time.Now().Add(-1 * time.Minute),
		UpdatedAt:             time.Now().Add(-1 * time.Minute),
	}
	require.NoError(t, txRepo.Save(ctx, recentTx))

	prov := &testReconcileProvider{
		paymentStatuses: map[string]portout.PaymentProviderStatus{
			"prov-recent-1": portout.ProviderStatusReceived, // provider says RECEIVED, but we shouldn't touch it
		},
	}

	uc := ucpayment.NewReconcilePspTransactions(txRepo, prov)

	// Threshold: 24h ago — recentTx (1min old) is NOT older than threshold.
	threshold := time.Now().Add(-24 * time.Hour)
	result, err := uc.Execute(ctx, portin.ReconcileFilter{From: threshold})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Checked, "no transactions should be checked (none older than threshold)")
	assert.Equal(t, int64(0), result.Updated)

	// The recent transaction must remain PENDING.
	notUpdated, err := txRepo.FindByID(ctx, recentTx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusPending, notUpdated.Status,
		"recent transaction must not be touched by reconcile")
}

// TestReconcilePsp_Integration_SkipsTransactionWithoutProviderID verifies that
// transactions without a ProviderTransactionID are skipped gracefully.
func TestReconcilePsp_Integration_SkipsTransactionWithoutProviderID(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	txRepo := mongodb.NewPaymentTransactionRepository(client)

	noProviderTx := &domain.PaymentTransaction{
		ID:                    "tx-noprov-1",
		UserID:                "user-noprov-1",
		AmountCents:           1000,
		PaymentMethod:         domain.PaymentMethodPix,
		Provider:              domain.PaymentProviderMock,
		Status:                domain.TransactionStatusPending,
		ProviderTransactionID: "", // no provider ID
		CreatedAt:             time.Now().Add(-48 * time.Hour),
		UpdatedAt:             time.Now().Add(-48 * time.Hour),
	}
	require.NoError(t, txRepo.Save(ctx, noProviderTx))

	prov := &testReconcileProvider{}
	uc := ucpayment.NewReconcilePspTransactions(txRepo, prov)

	threshold := time.Now().Add(-24 * time.Hour)
	result, err := uc.Execute(ctx, portin.ReconcileFilter{From: threshold})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Checked)
	assert.Equal(t, int64(0), result.Updated, "transaction without provider ID must be skipped")

	// Transaction remains PENDING.
	notUpdated, err := txRepo.FindByID(ctx, noProviderTx.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusPending, notUpdated.Status)
}

// ─── CancelParticipantInstallments use case integration tests ─────────────────

// mockEventoRepository is a minimal mock for EventoRepository used in
// CancelParticipantInstallments tests.
type mockEventoRepository struct {
	mock.Mock
}

func (m *mockEventoRepository) FindByID(_ context.Context, id string) (*domain_event.Evento, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain_event.Evento), args.Error(1)
}

func (m *mockEventoRepository) FindByUsuarioID(_ context.Context, uid string, page, pageSize int) ([]domain_event.Evento, int64, error) {
	return nil, 0, nil
}

func (m *mockEventoRepository) FindByUsuarioIDCursor(_ context.Context, uid string, f portout.EventoQueryFiltros) (portout.EventosCursorPage, error) {
	return portout.EventosCursorPage{}, nil
}

func (m *mockEventoRepository) FindAll(_ context.Context, page, pageSize int) ([]domain_event.Evento, int64, error) {
	return nil, 0, nil
}

func (m *mockEventoRepository) Save(_ context.Context, e *domain_event.Evento) (*domain_event.Evento, error) {
	return e, nil
}

func (m *mockEventoRepository) Update(_ context.Context, e *domain_event.Evento) (*domain_event.Evento, error) {
	return e, nil
}

func (m *mockEventoRepository) DeleteByID(_ context.Context, id string) error {
	return nil
}

func (m *mockEventoRepository) AddConvidados(_ context.Context, id string, c []domain_event.Convidado) error {
	return nil
}
func (m *mockEventoRepository) FindAllByIDs(_ context.Context, _ []string) ([]domain_event.Evento, error) {
	return nil, nil
}
func (m *mockEventoRepository) UpdateFase(_ context.Context, _ string, _ domain_event.EventoFase) error {
	return nil
}
func (m *mockEventoRepository) UpdatePoliticaConvidados(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockEventoRepository) AddImagens(_ context.Context, _ string, _ []domain_event.EventoImagem) error {
	return nil
}
func (m *mockEventoRepository) UpdateDetalhes(_ context.Context, e *domain_event.Evento) (*domain_event.Evento, error) {
	return e, nil
}

// TestCancelParticipantInstallments_Integration_OnlyPendingAndOverdue verifies
// that the CancelParticipantInstallments use case cancels only PENDING and
// OVERDUE installments for the given participant, leaving PAID and CANCELLED
// installments untouched.
//
// Assertions (spec-169 §4 Fase 4 CancelParticipantInstallments):
//   - Returns count == 2 (PENDING + OVERDUE).
//   - PENDING and OVERDUE installments become CANCELLED.
//   - PAID installment remains PAID.
//   - Already-CANCELLED installment count is not double-counted.
func TestCancelParticipantInstallments_Integration_OnlyPendingAndOverdue(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	instRepo := mongodb.NewPaymentInstallmentRepository(client)

	const (
		organizerID   = "organizer-cancel-1"
		eventID       = "evt-cancel-uc-1"
		participantID = "part-cancel-1"
	)

	// Seed installments with all statuses.
	statuses := []struct {
		id     string
		status domain.InstallmentStatus
	}{
		{"ci-pending-1", domain.InstallmentStatusPending},
		{"ci-overdue-1", domain.InstallmentStatusOverdue},
		{"ci-paid-1", domain.InstallmentStatusPaid},
		{"ci-cancelled-1", domain.InstallmentStatusCancelled},
	}
	for _, s := range statuses {
		require.NoError(t, instRepo.Save(ctx, &domain.PaymentInstallment{
			ID:            s.id,
			EventID:       eventID,
			ParticipantID: participantID,
			AmountCents:   3000,
			DueDate:       time.Now().Add(30 * 24 * time.Hour),
			Status:        s.status,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}))
	}

	// Mock evento repo: organizerID owns the event.
	evRepo := new(mockEventoRepository)
	evRepo.On("FindByID", eventID).Return(&domain_event.Evento{
		ID:        eventID,
		UsuarioID: organizerID,
	}, nil)

	// Mock participant repo — not exercised in this path.
	partRepoMock := new(mockInstParticipanteRepo)

	uc := ucpayment.NewCancelParticipantInstallments(instRepo, partRepoMock, evRepo)

	count, err := uc.Execute(ctx, portin.CancelParticipantInstallmentsInput{
		EventID:       eventID,
		ParticipantID: participantID,
		RequesterID:   organizerID,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count,
		"only PENDING + OVERDUE must be cancelled; got %d", count)

	// PENDING and OVERDUE are now CANCELLED.
	pend, _ := instRepo.FindByID(ctx, "ci-pending-1")
	assert.Equal(t, domain.InstallmentStatusCancelled, pend.Status,
		"PENDING installment must be CANCELLED")

	over, _ := instRepo.FindByID(ctx, "ci-overdue-1")
	assert.Equal(t, domain.InstallmentStatusCancelled, over.Status,
		"OVERDUE installment must be CANCELLED")

	// PAID remains PAID.
	paid, _ := instRepo.FindByID(ctx, "ci-paid-1")
	assert.Equal(t, domain.InstallmentStatusPaid, paid.Status,
		"PAID installment must remain PAID")

	// Already CANCELLED remains CANCELLED (not re-counted).
	alreadyCancelled, _ := instRepo.FindByID(ctx, "ci-cancelled-1")
	assert.Equal(t, domain.InstallmentStatusCancelled, alreadyCancelled.Status)
}

// TestCancelParticipantInstallments_Integration_NonOrganizerForbidden verifies
// that a non-organizer cannot cancel installments.
func TestCancelParticipantInstallments_Integration_NonOrganizerForbidden(t *testing.T) {
	client := testhelper.StartMongo(t)
	ctx := context.Background()
	require.NoError(t, migrations.RunV084CreatePaymentTransactionsIndexes(ctx, client.DB()))

	instRepo := mongodb.NewPaymentInstallmentRepository(client)

	const (
		organizerID = "organizer-cancel-2"
		eventID     = "evt-cancel-uc-2"
		intruderID  = "intruder-cancel-2"
	)

	evRepo := new(mockEventoRepository)
	evRepo.On("FindByID", eventID).Return(&domain_event.Evento{
		ID:        eventID,
		UsuarioID: organizerID,
	}, nil)

	partRepoMock := new(mockInstParticipanteRepo)
	uc := ucpayment.NewCancelParticipantInstallments(instRepo, partRepoMock, evRepo)

	_, err := uc.Execute(ctx, portin.CancelParticipantInstallmentsInput{
		EventID:       eventID,
		ParticipantID: "part-cancel-2",
		RequesterID:   intruderID, // NOT the organizer
	})

	require.Error(t, err)
	var ae *apierr.APIError
	require.ErrorAs(t, err, &ae, "expected APIError, got %T: %v", err, err)
	assert.Equal(t, 403, ae.Status, "non-organizer must receive 403 Forbidden")
}
