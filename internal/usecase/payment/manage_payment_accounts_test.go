package payment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mock PaymentAccountRepository ----

type mockAccountRepo struct{ mock.Mock }

func (m *mockAccountRepo) FindByID(ctx context.Context, id string) (*domain.PaymentAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentAccount), args.Error(1)
}

func (m *mockAccountRepo) FindByUserID(ctx context.Context, userID string) ([]*domain.PaymentAccount, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentAccount), args.Error(1)
}

func (m *mockAccountRepo) FindDefaultByUserID(ctx context.Context, userID string) (*domain.PaymentAccount, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentAccount), args.Error(1)
}

func (m *mockAccountRepo) Save(ctx context.Context, acct *domain.PaymentAccount) error {
	args := m.Called(ctx, acct)
	return args.Error(0)
}

func (m *mockAccountRepo) Update(ctx context.Context, acct *domain.PaymentAccount) error {
	args := m.Called(ctx, acct)
	return args.Error(0)
}

func (m *mockAccountRepo) SetDefault(ctx context.Context, userID, accountID string) error {
	args := m.Called(ctx, userID, accountID)
	return args.Error(0)
}

func (m *mockAccountRepo) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// ---- helpers ----

func pixAccount(id, userID string) *domain.PaymentAccount {
	return &domain.PaymentAccount{
		ID:          id,
		UserID:      userID,
		AccountType: domain.AccountTypePix,
		PixKeyType:  domain.PixKeyTypeCPF,
		PixKey:      "12345678901",
		IsDefault:   false,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// ---- TestManagePaymentAccounts_List ----

func TestManagePaymentAccounts_List_ReturnsAccounts(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	accounts := []*domain.PaymentAccount{pixAccount("acct-1", "usr-1"), pixAccount("acct-2", "usr-1")}
	repo.On("FindByUserID", mock.Anything, "usr-1").Return(accounts, nil)

	got, err := uc.List(context.Background(), "usr-1")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	repo.AssertExpectations(t)
}

func TestManagePaymentAccounts_List_Empty(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	repo.On("FindByUserID", mock.Anything, "usr-1").Return([]*domain.PaymentAccount{}, nil)

	got, err := uc.List(context.Background(), "usr-1")
	require.NoError(t, err)
	assert.Empty(t, got)
}

// ---- TestManagePaymentAccounts_Create ----

func TestManagePaymentAccounts_Create_FirstAccountBecomesDefault(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	// No existing accounts → first account becomes default.
	repo.On("FindByUserID", mock.Anything, "usr-1").Return([]*domain.PaymentAccount{}, nil)
	repo.On("Save", mock.Anything, mock.MatchedBy(func(a *domain.PaymentAccount) bool {
		return a.IsDefault == true && a.IsActive == true
	})).Return(nil)

	got, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:      "usr-1",
		AccountType: domain.AccountTypePix,
		PixKeyType:  domain.PixKeyTypeCPF,
		PixKey:      "12345678901",
	})
	require.NoError(t, err)
	assert.True(t, got.IsDefault, "first account must be default")
	assert.True(t, got.IsActive)
	repo.AssertExpectations(t)
}

func TestManagePaymentAccounts_Create_SecondAccountNotDefault(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	existing := []*domain.PaymentAccount{pixAccount("acct-1", "usr-1")}
	repo.On("FindByUserID", mock.Anything, "usr-1").Return(existing, nil)
	repo.On("Save", mock.Anything, mock.MatchedBy(func(a *domain.PaymentAccount) bool {
		return a.IsDefault == false
	})).Return(nil)

	got, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:      "usr-1",
		AccountType: domain.AccountTypePix,
		PixKeyType:  domain.PixKeyTypeEmail,
		PixKey:      "user@example.com",
	})
	require.NoError(t, err)
	assert.False(t, got.IsDefault, "second account must NOT be auto-default")
	repo.AssertExpectations(t)
}

func TestManagePaymentAccounts_Create_PIXMissingKey_Returns400(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	_, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:      "usr-1",
		AccountType: domain.AccountTypePix,
		PixKeyType:  domain.PixKeyTypeCPF,
		PixKey:      "", // missing
	})
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok, "error must be *apierr.APIError")
	assert.Equal(t, 400, ae.Status)
}

func TestManagePaymentAccounts_Create_PIXMissingKeyType_Returns400(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	_, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:      "usr-1",
		AccountType: domain.AccountTypePix,
		PixKeyType:  "", // missing
		PixKey:      "12345678901",
	})
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 400, ae.Status)
}

func TestManagePaymentAccounts_Create_BankAccountMissingFields_Returns400(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	_, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:        "usr-1",
		AccountType:   domain.AccountTypeBankAccount,
		AccountNumber: "", // missing
		// AccountHolderName: "", // also missing
	})
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 400, ae.Status)
}

func TestManagePaymentAccounts_Create_BankAccount_OK(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	repo.On("FindByUserID", mock.Anything, "usr-1").Return([]*domain.PaymentAccount{}, nil)
	repo.On("Save", mock.Anything, mock.AnythingOfType("*payment.PaymentAccount")).Return(nil)

	got, err := uc.Create(context.Background(), portin.CreatePaymentAccountInput{
		UserID:            "usr-1",
		AccountType:       domain.AccountTypeBankAccount,
		AccountHolderName: "João",
		AccountNumber:     "12345",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.AccountTypeBankAccount, got.AccountType)
	assert.True(t, got.IsDefault) // first account
}

// ---- TestManagePaymentAccounts_Update ----

func TestManagePaymentAccounts_Update_OwnerCanUpdate(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-1")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*payment.PaymentAccount")).Return(nil)

	newKey := "user@test.com"
	got, err := uc.Update(context.Background(), "acct-1", "usr-1", portin.UpdatePaymentAccountInput{
		PixKey: &newKey,
	})
	require.NoError(t, err)
	assert.Equal(t, "user@test.com", got.PixKey)
	repo.AssertExpectations(t)
}

func TestManagePaymentAccounts_Update_OtherUserForbidden(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)

	_, err := uc.Update(context.Background(), "acct-1", "usr-other", portin.UpdatePaymentAccountInput{})
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 403, ae.Status, "other user must receive 403")
}

// ---- TestManagePaymentAccounts_SetDefault ----

func TestManagePaymentAccounts_SetDefault_CallsRepo(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-1")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)
	// Repo SetDefault atomically clears others and sets this one.
	repo.On("SetDefault", mock.Anything, "usr-1", "acct-1").Return(nil)

	err := uc.SetDefault(context.Background(), "acct-1", "usr-1")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestManagePaymentAccounts_SetDefault_OtherUserForbidden(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)

	err := uc.SetDefault(context.Background(), "acct-1", "usr-other")
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 403, ae.Status)
}

// ---- TestManagePaymentAccounts_Delete ----

func TestManagePaymentAccounts_Delete_SoftDeleteCalled(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-1")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)
	repo.On("DeleteByID", mock.Anything, "acct-1").Return(nil)

	err := uc.Delete(context.Background(), "acct-1", "usr-1")
	require.NoError(t, err)
	// DeleteByID was called (soft delete — document still exists in DB but is_active=false)
	repo.AssertCalled(t, "DeleteByID", mock.Anything, "acct-1")
}

func TestManagePaymentAccounts_Delete_OtherUserForbidden(t *testing.T) {
	repo := new(mockAccountRepo)
	uc := ucpayment.NewManagePaymentAccounts(repo)

	acct := pixAccount("acct-1", "usr-owner")
	repo.On("FindByID", mock.Anything, "acct-1").Return(acct, nil)

	err := uc.Delete(context.Background(), "acct-1", "usr-other")
	require.Error(t, err)
	ae, ok := err.(*apierr.APIError)
	require.True(t, ok)
	assert.Equal(t, 403, ae.Status)
	// Must NOT call DeleteByID for unauthorized requests.
	repo.AssertNotCalled(t, "DeleteByID", mock.Anything, mock.Anything)
}
