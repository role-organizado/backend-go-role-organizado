package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Mock use cases ───────────────────────────────────────────────────────────

// mockManageAccounts implements portin.ManagePaymentAccountsUseCase.
type mockManageAccounts struct {
	listFn       func(ctx context.Context, userID string) ([]*domain.PaymentAccount, error)
	createFn     func(ctx context.Context, in portin.CreatePaymentAccountInput) (*domain.PaymentAccount, error)
	updateFn     func(ctx context.Context, id, userID string, in portin.UpdatePaymentAccountInput) (*domain.PaymentAccount, error)
	setDefaultFn func(ctx context.Context, id, userID string) error
	deleteFn     func(ctx context.Context, id, userID string) error
}

func (m *mockManageAccounts) List(ctx context.Context, userID string) ([]*domain.PaymentAccount, error) {
	return m.listFn(ctx, userID)
}
func (m *mockManageAccounts) Create(ctx context.Context, in portin.CreatePaymentAccountInput) (*domain.PaymentAccount, error) {
	return m.createFn(ctx, in)
}
func (m *mockManageAccounts) Update(ctx context.Context, id, userID string, in portin.UpdatePaymentAccountInput) (*domain.PaymentAccount, error) {
	return m.updateFn(ctx, id, userID, in)
}
func (m *mockManageAccounts) SetDefault(ctx context.Context, id, userID string) error {
	return m.setDefaultFn(ctx, id, userID)
}
func (m *mockManageAccounts) Delete(ctx context.Context, id, userID string) error {
	return m.deleteFn(ctx, id, userID)
}

// mockValidatePix implements portin.ValidatePixKeyUseCase.
type mockValidatePix struct {
	executeFn func(ctx context.Context, userID, pixKey string) (*portin.ValidatePixKeyResult, error)
}

func (m *mockValidatePix) Execute(ctx context.Context, userID, pixKey string) (*portin.ValidatePixKeyResult, error) {
	return m.executeFn(ctx, userID, pixKey)
}

// mockManageSavedCards implements portin.ManageSavedCardsUseCase.
type mockManageSavedCards struct {
	listFn       func(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error)
	setDefaultFn func(ctx context.Context, cardID, userID string) error
	deleteFn     func(ctx context.Context, cardID, userID string) error
}

func (m *mockManageSavedCards) List(ctx context.Context, userID string) ([]*domain.SavedCreditCard, error) {
	return m.listFn(ctx, userID)
}
func (m *mockManageSavedCards) SetDefault(ctx context.Context, cardID, userID string) error {
	return m.setDefaultFn(ctx, cardID, userID)
}
func (m *mockManageSavedCards) Delete(ctx context.Context, cardID, userID string) error {
	return m.deleteFn(ctx, cardID, userID)
}

// ─── Router helper ────────────────────────────────────────────────────────────

func newPaymentMethodsRouter(
	accounts portin.ManagePaymentAccountsUseCase,
	pix portin.ValidatePixKeyUseCase,
	cards portin.ManageSavedCardsUseCase,
) http.Handler {
	h := handler.NewPaymentMethodsHandler(accounts, pix, cards)
	r := chi.NewRouter()
	h.RegisterPaymentMethodsRoutes(r)
	return r
}

// ctxWithUser injects userID into the request context (mirrors middleware.WithUserIDContext).
func ctxWithUser(r *http.Request, userID string) *http.Request {
	ctx := middleware.WithUserIDContext(r.Context(), userID)
	return r.WithContext(ctx)
}

// stubAccount returns a minimal PaymentAccount for testing.
func stubAccount(id, userID string, isDefault bool) *domain.PaymentAccount {
	return &domain.PaymentAccount{
		ID:          id,
		UserID:      userID,
		AccountType: domain.AccountTypePix,
		PixKeyType:  domain.PixKeyTypeCPF,
		PixKey:      "12345678901",
		IsDefault:   isDefault,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// ─── PaymentAccount handler tests ────────────────────────────────────────────

func TestPaymentMethods_List_Returns200(t *testing.T) {
	accounts := []*domain.PaymentAccount{
		stubAccount("a1", "usr-1", true),
		stubAccount("a2", "usr-1", false),
	}
	mock := &mockManageAccounts{
		listFn: func(_ context.Context, _ string) ([]*domain.PaymentAccount, error) {
			return accounts, nil
		},
	}

	r := newPaymentMethodsRouter(mock, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payment-methods", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)

	var resp []map[string]interface{}
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.Len(t, resp, 2)
}

func TestPaymentMethods_Create_Returns201(t *testing.T) {
	created := stubAccount("new-id", "usr-1", true)
	mockAcc := &mockManageAccounts{
		createFn: func(_ context.Context, in portin.CreatePaymentAccountInput) (*domain.PaymentAccount, error) {
			assert.Equal(t, "usr-1", in.UserID)
			assert.Equal(t, domain.AccountTypePix, in.AccountType)
			return created, nil
		},
	}

	body, _ := json.Marshal(map[string]string{
		"accountType": "PIX",
		"pixKeyType":  "CPF",
		"pixKey":      "12345678901",
	})
	r := newPaymentMethodsRouter(mockAcc, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-methods", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusCreated, rw.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.Equal(t, "new-id", resp["id"])
}

func TestPaymentMethods_Update_Returns200(t *testing.T) {
	updated := stubAccount("acct-1", "usr-1", false)
	mockAcc := &mockManageAccounts{
		updateFn: func(_ context.Context, id, userID string, _ portin.UpdatePaymentAccountInput) (*domain.PaymentAccount, error) {
			assert.Equal(t, "acct-1", id)
			assert.Equal(t, "usr-1", userID)
			return updated, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"pixKey": "user@example.com"})
	r := newPaymentMethodsRouter(mockAcc, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/payment-methods/acct-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
}

func TestPaymentMethods_Update_Forbidden_Returns403(t *testing.T) {
	mockAcc := &mockManageAccounts{
		updateFn: func(_ context.Context, _, _ string, _ portin.UpdatePaymentAccountInput) (*domain.PaymentAccount, error) {
			return nil, apierr.Forbidden("acesso negado")
		},
	}

	body, _ := json.Marshal(map[string]string{"pixKey": "anything"})
	r := newPaymentMethodsRouter(mockAcc, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/payment-methods/acct-other", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(req, "usr-other")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusForbidden, rw.Code)
}

func TestPaymentMethods_SetDefault_Returns200(t *testing.T) {
	mockAcc := &mockManageAccounts{
		setDefaultFn: func(_ context.Context, id, userID string) error {
			assert.Equal(t, "acct-1", id)
			assert.Equal(t, "usr-1", userID)
			return nil
		},
	}

	r := newPaymentMethodsRouter(mockAcc, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-methods/acct-1/set-default", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	var resp map[string]bool
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.True(t, resp["success"])
}

func TestPaymentMethods_Delete_Returns204(t *testing.T) {
	mockAcc := &mockManageAccounts{
		deleteFn: func(_ context.Context, id, userID string) error {
			assert.Equal(t, "acct-1", id)
			assert.Equal(t, "usr-1", userID)
			return nil
		},
	}

	r := newPaymentMethodsRouter(mockAcc, &mockValidatePix{}, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/payment-methods/acct-1", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusNoContent, rw.Code)
	assert.Empty(t, rw.Body.String())
}

// ─── PIX validate handler tests ───────────────────────────────────────────────

func TestPaymentMethods_ValidatePix_ValidKey(t *testing.T) {
	mockPix := &mockValidatePix{
		executeFn: func(_ context.Context, _, key string) (*portin.ValidatePixKeyResult, error) {
			return &portin.ValidatePixKeyResult{Valid: true, KeyType: "CPF", Key: key}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"key": "12345678901"})
	r := newPaymentMethodsRouter(&mockManageAccounts{}, mockPix, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-methods/pix/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.True(t, resp["valid"].(bool))
	assert.Equal(t, "CPF", resp["keyType"])
}

func TestPaymentMethods_ValidatePix_InvalidKey(t *testing.T) {
	mockPix := &mockValidatePix{
		executeFn: func(_ context.Context, _, _ string) (*portin.ValidatePixKeyResult, error) {
			return &portin.ValidatePixKeyResult{Valid: false, KeyType: "", Key: "bad"}, nil
		},
	}

	body, _ := json.Marshal(map[string]string{"key": "bad"})
	r := newPaymentMethodsRouter(&mockManageAccounts{}, mockPix, &mockManageSavedCards{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payment-methods/pix/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.False(t, resp["valid"].(bool))
}

// ─── SavedCard handler tests ──────────────────────────────────────────────────

func stubCard(id, userID string, isDefault bool) *domain.SavedCreditCard {
	return &domain.SavedCreditCard{
		ID:             id,
		UserID:         userID,
		LastFourDigits: "4242",
		Brand:          domain.CardBrandVisa,
		HolderName:     "Test User",
		ExpirationDate: "12/2025",
		IsDefault:      isDefault,
		IsActive:       true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func TestSavedCard_List_Returns200(t *testing.T) {
	cards := []*domain.SavedCreditCard{
		stubCard("card-1", "usr-1", true),
		stubCard("card-2", "usr-1", false),
	}
	mockCards := &mockManageSavedCards{
		listFn: func(_ context.Context, _ string) ([]*domain.SavedCreditCard, error) {
			return cards, nil
		},
	}

	r := newPaymentMethodsRouter(&mockManageAccounts{}, &mockValidatePix{}, mockCards)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/saved-cards", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	var resp []map[string]interface{}
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&resp))
	assert.Len(t, resp, 2)
}

func TestSavedCard_SetDefault_Returns200(t *testing.T) {
	called := false
	mockCards := &mockManageSavedCards{
		setDefaultFn: func(_ context.Context, cardID, userID string) error {
			called = true
			assert.Equal(t, "card-1", cardID)
			assert.Equal(t, "usr-1", userID)
			return nil
		},
	}

	r := newPaymentMethodsRouter(&mockManageAccounts{}, &mockValidatePix{}, mockCards)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/saved-cards/card-1/set-default", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusOK, rw.Code)
	assert.True(t, called)
}

func TestSavedCard_Delete_Returns204(t *testing.T) {
	called := false
	mockCards := &mockManageSavedCards{
		deleteFn: func(_ context.Context, cardID, userID string) error {
			called = true
			assert.Equal(t, "card-1", cardID)
			return nil
		},
	}

	r := newPaymentMethodsRouter(&mockManageAccounts{}, &mockValidatePix{}, mockCards)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/saved-cards/card-1", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusNoContent, rw.Code)
	assert.True(t, called)
}

func TestSavedCard_Delete_ServiceError_Returns500(t *testing.T) {
	mockCards := &mockManageSavedCards{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierr.Internal("db error")
		},
	}

	r := newPaymentMethodsRouter(&mockManageAccounts{}, &mockValidatePix{}, mockCards)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/saved-cards/card-x", nil)
	req = ctxWithUser(req, "usr-1")
	rw := httptest.NewRecorder()
	r.ServeHTTP(rw, req)

	assert.Equal(t, http.StatusInternalServerError, rw.Code)
}
