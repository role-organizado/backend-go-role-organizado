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

// ─── Mock: PaymentTransactionRepository ─────────────────────────────────────

type mockTxRepo struct{ mock.Mock }

func (m *mockTxRepo) Save(ctx context.Context, tx *domain.PaymentTransaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}
func (m *mockTxRepo) Update(ctx context.Context, tx *domain.PaymentTransaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}
func (m *mockTxRepo) FindByID(ctx context.Context, id string) (*domain.PaymentTransaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentTransaction), args.Error(1)
}
func (m *mockTxRepo) FindByIdempotencyKey(ctx context.Context, key string) (*domain.PaymentTransaction, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentTransaction), args.Error(1)
}
func (m *mockTxRepo) FindByProviderTransactionID(ctx context.Context, id string) (*domain.PaymentTransaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentTransaction), args.Error(1)
}
func (m *mockTxRepo) FindByUserID(ctx context.Context, userID string, filter portout.TransactionFilter) ([]*domain.PaymentTransaction, int64, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.PaymentTransaction), args.Get(1).(int64), args.Error(2)
}
func (m *mockTxRepo) FindPendingOlderThan(ctx context.Context, threshold time.Time) ([]*domain.PaymentTransaction, error) {
	args := m.Called(ctx, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentTransaction), args.Error(1)
}
func (m *mockTxRepo) FindCompletedByEventID(ctx context.Context, eventID string, since time.Time) ([]*domain.PaymentTransaction, error) {
	args := m.Called(ctx, eventID, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentTransaction), args.Error(1)
}

// ─── Mock: AsaasCustomerLinkRepository ──────────────────────────────────────

type mockCustomerLinkRepo struct{ mock.Mock }

func (m *mockCustomerLinkRepo) FindByUserID(ctx context.Context, userID string) (*domain.AsaasCustomerLink, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AsaasCustomerLink), args.Error(1)
}
func (m *mockCustomerLinkRepo) Save(ctx context.Context, link *domain.AsaasCustomerLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}
func (m *mockCustomerLinkRepo) Update(ctx context.Context, link *domain.AsaasCustomerLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

// ─── Mock: UsuarioRepository ─────────────────────────────────────────────────

type mockUsuarioRepo struct{ mock.Mock }

func (m *mockUsuarioRepo) FindByID(ctx context.Context, id string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByEmail(ctx context.Context, email string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindByProviderID(ctx context.Context, provider, providerUserID string) (*authdomain.Usuario, error) {
	args := m.Called(ctx, provider, providerUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Save(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) Update(ctx context.Context, u *authdomain.Usuario) (*authdomain.Usuario, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*authdomain.Usuario), args.Error(1)
}
func (m *mockUsuarioRepo) FindAll(ctx context.Context, page, pageSize int) ([]authdomain.Usuario, int64, error) {
	args := m.Called(ctx, page, pageSize)
	return args.Get(0).([]authdomain.Usuario), args.Get(1).(int64), args.Error(2)
}
func (m *mockUsuarioRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ─── Mock: SavedCreditCardRepository ────────────────────────────────────────

type mockSavedCardRepo struct{ mock.Mock }

func (m *mockSavedCardRepo) FindByID(ctx context.Context, id string) (*domain.SavedCreditCard, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SavedCreditCard), args.Error(1)
}
func (m *mockSavedCardRepo) FindByUserID(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SavedCreditCard), args.Error(1)
}
func (m *mockSavedCardRepo) FindDefaultByUserID(ctx context.Context, userID string) (*domain.SavedCreditCard, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SavedCreditCard), args.Error(1)
}
func (m *mockSavedCardRepo) Save(ctx context.Context, card *domain.SavedCreditCard) error {
	args := m.Called(ctx, card)
	return args.Error(0)
}
func (m *mockSavedCardRepo) Update(ctx context.Context, card *domain.SavedCreditCard) error {
	args := m.Called(ctx, card)
	return args.Error(0)
}
func (m *mockSavedCardRepo) SetDefault(ctx context.Context, userID, cardID string) error {
	args := m.Called(ctx, userID, cardID)
	return args.Error(0)
}
func (m *mockSavedCardRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ─── Mock: PaymentProvider ───────────────────────────────────────────────────

type mockProvider struct{ mock.Mock }

func (m *mockProvider) CreateOrGetCustomer(ctx context.Context, userID, name, cpf, email string) (*portout.ProviderCustomer, error) {
	args := m.Called(ctx, userID, name, cpf, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderCustomer), args.Error(1)
}
func (m *mockProvider) CreatePayment(ctx context.Context, req *portout.CreatePaymentRequest) (*portout.ProviderPayment, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderPayment), args.Error(1)
}
func (m *mockProvider) GetPayment(ctx context.Context, providerID string) (*portout.ProviderPayment, error) {
	args := m.Called(ctx, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderPayment), args.Error(1)
}
func (m *mockProvider) GetPixQrCode(ctx context.Context, providerID string) (*portout.ProviderPixQrCode, error) {
	args := m.Called(ctx, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderPixQrCode), args.Error(1)
}
func (m *mockProvider) GetBoletoIdentificationField(ctx context.Context, providerID string) (*portout.ProviderIdentificationField, error) {
	args := m.Called(ctx, providerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderIdentificationField), args.Error(1)
}
func (m *mockProvider) TokenizeCreditCard(ctx context.Context, req *portout.TokenizeCreditCardRequest) (*portout.ProviderCardToken, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portout.ProviderCardToken), args.Error(1)
}
func (m *mockProvider) SimulateSandboxReceive(ctx context.Context, providerID string) error {
	args := m.Called(ctx, providerID)
	return args.Error(0)
}

// ─── Mock: FeePolicyResolver ─────────────────────────────────────────────────

type mockFeePolicy struct{ mock.Mock }

func (m *mockFeePolicy) ResolveSnapshot(ctx context.Context, eventID string) (domain.FeePolicySnapshot, error) {
	args := m.Called(ctx, eventID)
	return args.Get(0).(domain.FeePolicySnapshot), args.Error(1)
}

// ─── Mock: PaymentCommitmentWriter ──────────────────────────────────────────

type mockSubledger struct{ mock.Mock }

func (m *mockSubledger) AppendPaymentCommitment(ctx context.Context, tx *domain.PaymentTransaction, fees domain.FeeCalculationResult, snapshot domain.FeePolicySnapshot) error {
	args := m.Called(ctx, tx, fees, snapshot)
	return args.Error(0)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

var globalSnapshot = domain.FeePolicySnapshot{
	FeePolicySource: "GLOBAL",
	PspFeePercent:   1.99,
	Version:         "global-v1",
}

func buildUseCase(t *testing.T) (
	*ucpayment.ProcessPayment,
	*mockTxRepo,
	*mockCustomerLinkRepo,
	*mockUsuarioRepo,
	*mockSavedCardRepo,
	*mockProvider,
	*mockFeePolicy,
	*mockSubledger,
) {
	t.Helper()
	txRepo := new(mockTxRepo)
	linkRepo := new(mockCustomerLinkRepo)
	usuRepo := new(mockUsuarioRepo)
	cardRepo := new(mockSavedCardRepo)
	prov := new(mockProvider)
	fee := new(mockFeePolicy)
	sub := new(mockSubledger)

	uc := ucpayment.NewProcessPayment(
		txRepo, linkRepo, usuRepo, cardRepo,
		prov, domain.PaymentProviderMock,
		fee, sub,
	)
	return uc, txRepo, linkRepo, usuRepo, cardRepo, prov, fee, sub
}

// stubHappyPath wires the most common sequence: no cached customer, create customer,
// fee snapshot, save tx, subledger, createPayment.
func stubHappyPath(
	txRepo *mockTxRepo,
	linkRepo *mockCustomerLinkRepo,
	usuRepo *mockUsuarioRepo,
	prov *mockProvider,
	fee *mockFeePolicy,
	sub *mockSubledger,
) *portout.ProviderPayment {
	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(nil, apierr.NotFound("link", "user-1"))
	usuRepo.On("FindByID", mock.Anything, "user-1").Return(&authdomain.Usuario{
		ID: "user-1", Nome: "Test User", Email: "test@test.com", CPF: "12345678901",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, "user-1", mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "asaas-cust-1"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-1").Return(globalSnapshot, nil)

	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	provPayment := &portout.ProviderPayment{
		ID:     "asaas-pay-1",
		Status: portout.ProviderStatusPending,
	}
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(provPayment, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	return provPayment
}

// ─── TestProcessPayment ───────────────────────────────────────────────────────

func TestProcessPayment_Idempotency(t *testing.T) {
	uc, txRepo, _, _, _, _, _, _ := buildUseCase(t)

	existing := &domain.PaymentTransaction{
		ID:     "tx-existing",
		UserID: "user-1",
		Status: domain.TransactionStatusPending,
	}
	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-key-1").Return(existing, nil)

	input := portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-1",
		AmountCents:    500,
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-key-1",
		CPF:            "12345678901",
	}

	got, err := uc.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "tx-existing", got.ID)

	// Provider must NOT be called a second time.
	txRepo.AssertNumberOfCalls(t, "FindByIdempotencyKey", 1)
	txRepo.AssertNotCalled(t, "Save")
}

func TestProcessPayment_ValidationAmountTooSmall(t *testing.T) {
	uc, txRepo, _, _, _, _, _, _ := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, mock.Anything).
		Return(nil, apierr.NotFound("tx", ""))

	_, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-1",
		AmountCents:    50, // below minimum of 100
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-2",
		CPF:            "12345678901",
	})
	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae), "expected APIError, got %T: %v", err, err)
	assert.Equal(t, 400, ae.Status)
}

func TestProcessPayment_ValidationCPFMissing(t *testing.T) {
	uc, txRepo, _, _, _, _, _, _ := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, mock.Anything).
		Return(nil, apierr.NotFound("tx", ""))

	for _, method := range []domain.PaymentMethod{
		domain.PaymentMethodPix,
		domain.PaymentMethodBoleto,
		domain.PaymentMethodCreditCard,
	} {
		t.Run(string(method), func(t *testing.T) {
			_, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
				UserID:         "user-1",
				EventID:        "evt-1",
				AmountCents:    500,
				Method:         method,
				IdempotencyKey: "idem-cpf-" + string(method),
				CPF:            "", // intentionally empty
			})
			require.Error(t, err)
			var ae *apierr.APIError
			require.True(t, errors.As(err, &ae), "expected APIError for %s", method)
			assert.Equal(t, 400, ae.Status)
		})
	}
}

func TestProcessPayment_PixFullFlow(t *testing.T) {
	uc, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildUseCase(t)

	// Idempotency miss
	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-pix-1").
		Return(nil, apierr.NotFound("tx", "idem-pix-1"))

	stubHappyPath(txRepo, linkRepo, usuRepo, prov, fee, sub)

	// PIX-specific: GetPixQrCode
	expireTime := time.Now().Add(30 * time.Minute)
	prov.On("GetPixQrCode", mock.Anything, "asaas-pay-1").Return(&portout.ProviderPixQrCode{
		EncodedImage:   "base64PNG",
		Payload:        "00020126...",
		ExpirationDate: expireTime,
	}, nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-1",
		AmountCents:    1000,
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-pix-1",
		CPF:            "12345678901",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusPending, got.Status)
	assert.Equal(t, "base64PNG", got.Metadata.PixQrCodeImage)
	assert.Equal(t, "00020126...", got.Metadata.PixQrCodeText)
	assert.NotNil(t, got.ExpiresAt)

	txRepo.AssertCalled(t, "Save", mock.Anything, mock.Anything)
	txRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
	sub.AssertCalled(t, "AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessPayment_ProviderFailure_MarksTransactionFailed(t *testing.T) {
	uc, txRepo, linkRepo, usuRepo, _, prov, fee, sub := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-fail").
		Return(nil, apierr.NotFound("tx", "idem-fail"))

	// Customer path
	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(nil, apierr.NotFound("link", "user-1"))
	usuRepo.On("FindByID", mock.Anything, "user-1").Return(&authdomain.Usuario{
		ID: "user-1", Nome: "User", Email: "u@test.com",
	}, nil)
	prov.On("CreateOrGetCustomer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&portout.ProviderCustomer{ID: "cust-1"}, nil)
	linkRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-1").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Provider returns an error
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(nil, errors.New("psp unavailable"))
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-1",
		AmountCents:    500,
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-fail",
		CPF:            "12345678901",
	})

	// Failure is encoded in the transaction; Execute must not return an error.
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusFailed, got.Status)
	assert.Contains(t, got.FailureReason, "psp unavailable")
	txRepo.AssertCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestProcessPayment_CustomerCached(t *testing.T) {
	uc, txRepo, linkRepo, _, _, prov, fee, sub := buildUseCase(t)

	txRepo.On("FindByIdempotencyKey", mock.Anything, "idem-cached").
		Return(nil, apierr.NotFound("tx", ""))

	// Cached customer link — no need to create a new one.
	linkRepo.On("FindByUserID", mock.Anything, "user-1").Return(&domain.AsaasCustomerLink{
		UserID:          "user-1",
		AsaasCustomerID: "asaas-cust-cached",
	}, nil)

	fee.On("ResolveSnapshot", mock.Anything, "evt-1").Return(globalSnapshot, nil)
	txRepo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)
	sub.On("AppendPaymentCommitment", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	prov.On("CreatePayment", mock.Anything, mock.Anything).Return(&portout.ProviderPayment{
		ID:     "asaas-pay-cached",
		Status: portout.ProviderStatusPending,
	}, nil)
	prov.On("GetPixQrCode", mock.Anything, "asaas-pay-cached").Return(&portout.ProviderPixQrCode{
		Payload:        "pix-payload",
		ExpirationDate: time.Now().Add(30 * time.Minute),
	}, nil)
	txRepo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentTransaction")).Return(nil)

	got, err := uc.Execute(context.Background(), portin.ProcessPaymentInput{
		UserID:         "user-1",
		EventID:        "evt-1",
		AmountCents:    200,
		Method:         domain.PaymentMethodPix,
		IdempotencyKey: "idem-cached",
		CPF:            "12345678901",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.TransactionStatusPending, got.Status)

	// CreateOrGetCustomer must NOT be called if the link is cached.
	prov.AssertNotCalled(t, "CreateOrGetCustomer")
}

// ─── TestGetTransaction ──────────────────────────────────────────────────────

func TestGetTransaction_Success(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewGetPaymentTransaction(txRepo)

	tx := &domain.PaymentTransaction{
		ID:     "tx-1",
		UserID: "user-1",
		Status: domain.TransactionStatusPending,
	}
	txRepo.On("FindByID", mock.Anything, "tx-1").Return(tx, nil)

	got, err := uc.Execute(context.Background(), "tx-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, "tx-1", got.ID)
}

func TestGetTransaction_Forbidden(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewGetPaymentTransaction(txRepo)

	tx := &domain.PaymentTransaction{
		ID:     "tx-1",
		UserID: "user-owner",
		Status: domain.TransactionStatusPending,
	}
	txRepo.On("FindByID", mock.Anything, "tx-1").Return(tx, nil)

	_, err := uc.Execute(context.Background(), "tx-1", "user-other")
	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, 403, ae.Status)
}

func TestGetTransaction_NotFound(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewGetPaymentTransaction(txRepo)

	txRepo.On("FindByID", mock.Anything, "tx-missing").
		Return(nil, apierr.NotFound("transaction", "tx-missing"))

	_, err := uc.Execute(context.Background(), "tx-missing", "user-1")
	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, 404, ae.Status)
}

// ─── TestListUserPayments ────────────────────────────────────────────────────

func TestListUserPayments_Success(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewListUserPayments(txRepo)

	txs := []*domain.PaymentTransaction{
		{ID: "tx-1", UserID: "user-1"},
		{ID: "tx-2", UserID: "user-1"},
	}
	txRepo.On("FindByUserID", mock.Anything, "user-1", mock.Anything).Return(txs, int64(2), nil)

	got, err := uc.Execute(context.Background(), "user-1", portin.ListUserPaymentsFilter{})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestListUserPayments_ListForRequester_Forbidden(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewListUserPayments(txRepo)

	_, err := uc.ListForRequester(context.Background(), "user-target", "user-other", portin.ListUserPaymentsFilter{})
	require.Error(t, err)
	var ae *apierr.APIError
	require.True(t, errors.As(err, &ae))
	assert.Equal(t, 403, ae.Status)

	// Repo must not be called.
	txRepo.AssertNotCalled(t, "FindByUserID")
}

func TestListUserPayments_ListForRequester_Owner(t *testing.T) {
	txRepo := new(mockTxRepo)
	uc := ucpayment.NewListUserPayments(txRepo)

	txs := []*domain.PaymentTransaction{{ID: "tx-1", UserID: "user-1"}}
	txRepo.On("FindByUserID", mock.Anything, "user-1", mock.Anything).Return(txs, int64(1), nil)

	got, err := uc.ListForRequester(context.Background(), "user-1", "user-1", portin.ListUserPaymentsFilter{})
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
