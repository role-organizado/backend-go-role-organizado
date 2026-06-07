package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/payment"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ─── Mock use cases ───────────────────────────────────────────────────────────

type mockListUserInstallmentsUC struct{ mock.Mock }

func (m *mockListUserInstallmentsUC) Execute(ctx context.Context, userID string, statusFilter *domain.InstallmentStatus) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, userID, statusFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

type mockListInstallmentsUC struct{ mock.Mock }

func (m *mockListInstallmentsUC) Execute(ctx context.Context, requesterID string, filter portin.ListInstallmentsFilter) ([]*domain.PaymentInstallment, error) {
	args := m.Called(ctx, requesterID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.PaymentInstallment), args.Error(1)
}

type mockGetInstallmentUC struct{ mock.Mock }

func (m *mockGetInstallmentUC) Execute(ctx context.Context, installmentID, requesterID string) (*domain.PaymentInstallment, error) {
	args := m.Called(ctx, installmentID, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.PaymentInstallment), args.Error(1)
}

type mockCancelInstallmentsUC struct{ mock.Mock }

func (m *mockCancelInstallmentsUC) Execute(ctx context.Context, in portin.CancelParticipantInstallmentsInput) (int64, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(int64), args.Error(1)
}

// ─── Test router builder ──────────────────────────────────────────────────────

func newTestInstallmentHandler(
	listUserUC portin.ListUserInstallmentsUseCase,
	listUC portin.ListInstallmentsUseCase,
	getUC portin.GetInstallmentUseCase,
	cancelUC portin.CancelParticipantInstallmentsUseCase,
) (*chi.Mux, *handler.InstallmentHandler) {
	r := chi.NewRouter()
	h := handler.NewInstallmentHandler(listUserUC, listUC, getUC, cancelUC)
	h.RegisterInstallmentRoutes(r)
	return r, h
}

// ─── GET /api/v1/installments ─────────────────────────────────────────────────

// TestInstallmentHandler_ListInstallments_MissingEventIdReturns400 ensures the
// handler itself enforces the eventId requirement before calling the use case.
func TestInstallmentHandler_ListInstallments_MissingEventIdReturns400(t *testing.T) {
	listUC := new(mockListInstallmentsUC)
	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), listUC,
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "BAD_REQUEST", body["code"])
	// Use case must NOT be called when eventId is missing.
	listUC.AssertNotCalled(t, "Execute")
}

// TestInstallmentHandler_ListInstallments_NonParticipantGets403 ensures the use-case
// Forbidden error is surfaced as 403 by the handler.
func TestInstallmentHandler_ListInstallments_NonParticipantGets403(t *testing.T) {
	listUC := new(mockListInstallmentsUC)
	listUC.On("Execute", mock.Anything, "user-1", portin.ListInstallmentsFilter{EventID: "event-1"}).
		Return(nil, apierr.Forbidden("acesso negado"))

	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), listUC,
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments?eventId=event-1", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestInstallmentHandler_ListInstallments_ReturnsInstallments tests happy path.
func TestInstallmentHandler_ListInstallments_ReturnsInstallments(t *testing.T) {
	listUC := new(mockListInstallmentsUC)
	inst := &domain.PaymentInstallment{
		ID:                "inst-1",
		EventID:           "event-1",
		ParticipantID:     "user-1",
		InstallmentNumber: 1,
		TotalInstallments: 1,
		AmountCents:       10000,
		Status:            domain.InstallmentStatusPending,
	}
	listUC.On("Execute", mock.Anything, "user-1", portin.ListInstallmentsFilter{EventID: "event-1"}).
		Return([]*domain.PaymentInstallment{inst}, nil)

	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), listUC,
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments?eventId=event-1", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, "inst-1", body[0]["id"])
	assert.Equal(t, "event-1", body[0]["eventId"])
	assert.EqualValues(t, 10000, body[0]["amountCents"])
}

// ─── GET /api/v1/installments/user ───────────────────────────────────────────

// TestInstallmentHandler_ListUserInstallments_Success tests basic success path.
func TestInstallmentHandler_ListUserInstallments_Success(t *testing.T) {
	listUserUC := new(mockListUserInstallmentsUC)
	inst := &domain.PaymentInstallment{
		ID:            "inst-user-1",
		EventID:       "event-1",
		ParticipantID: "user-1",
		AmountCents:   5000,
		Status:        domain.InstallmentStatusPaid,
	}
	listUserUC.On("Execute", mock.Anything, "user-1", (*domain.InstallmentStatus)(nil)).
		Return([]*domain.PaymentInstallment{inst}, nil)

	r, _ := newTestInstallmentHandler(
		listUserUC, new(mockListInstallmentsUC),
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments/user", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body, 1)
	assert.Equal(t, "inst-user-1", body[0]["id"])
}

// TestInstallmentHandler_ListUserInstallments_Unauthorized ensures 401 without JWT.
func TestInstallmentHandler_ListUserInstallments_Unauthorized(t *testing.T) {
	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), new(mockListInstallmentsUC),
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments/user", nil)
	// No user ID injected → no JWT
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ─── GET /api/v1/installments/{installmentId} ────────────────────────────────

// TestInstallmentHandler_GetInstallment_NotFound verifies 404 propagation.
func TestInstallmentHandler_GetInstallment_NotFound(t *testing.T) {
	getUC := new(mockGetInstallmentUC)
	getUC.On("Execute", mock.Anything, "bad-id", "user-1").
		Return(nil, apierr.NotFound("payment_installment", "bad-id"))

	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), new(mockListInstallmentsUC),
		getUC, new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments/bad-id", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestInstallmentHandler_GetInstallment_ResponseShape verifies the DTO shape.
func TestInstallmentHandler_GetInstallment_ResponseShape(t *testing.T) {
	getUC := new(mockGetInstallmentUC)
	inst := &domain.PaymentInstallment{
		ID:                "inst-abc",
		EventID:           "event-1",
		ParticipantID:     "participant-1",
		InstallmentNumber: 2,
		TotalInstallments: 4,
		AmountCents:       7500,
		Status:            domain.InstallmentStatusPending,
		TransactionID:     "tx-xyz",
		PaymentMethod:     "PIX",
	}
	getUC.On("Execute", mock.Anything, "inst-abc", "user-1").Return(inst, nil)

	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), new(mockListInstallmentsUC),
		getUC, new(mockCancelInstallmentsUC),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/installments/inst-abc", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "inst-abc", body["id"])
	assert.Equal(t, "event-1", body["eventId"])
	assert.EqualValues(t, 2, body["installmentNumber"])
	assert.EqualValues(t, 7500, body["amountCents"])
	assert.Equal(t, "PENDING", body["status"])
	assert.Equal(t, "tx-xyz", body["transactionId"])
	assert.Equal(t, "PIX", body["paymentMethod"])
}

// ─── POST /api/v1/installments/cancel ────────────────────────────────────────

// TestInstallmentHandler_CancelParticipantInstallments_Success tests happy path.
func TestInstallmentHandler_CancelParticipantInstallments_Success(t *testing.T) {
	cancelUC := new(mockCancelInstallmentsUC)
	cancelUC.On("Execute", mock.Anything, portin.CancelParticipantInstallmentsInput{
		EventID:       "event-1",
		ParticipantID: "participant-1",
		RequesterID:   "organizer-1",
	}).Return(int64(3), nil)

	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), new(mockListInstallmentsUC),
		new(mockGetInstallmentUC), cancelUC,
	)

	body := map[string]string{"eventId": "event-1", "participantId": "participant-1"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/installments/cancel", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "organizer-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.EqualValues(t, 3, resp["cancelled"])
	assert.Equal(t, "event-1", resp["eventId"])
	assert.Equal(t, "participant-1", resp["participantId"])
}

// TestInstallmentHandler_CancelParticipantInstallments_MissingFields returns 400.
func TestInstallmentHandler_CancelParticipantInstallments_MissingFields(t *testing.T) {
	r, _ := newTestInstallmentHandler(
		new(mockListUserInstallmentsUC), new(mockListInstallmentsUC),
		new(mockGetInstallmentUC), new(mockCancelInstallmentsUC),
	)

	// Missing participantId.
	body := map[string]string{"eventId": "event-1"}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/installments/cancel", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "organizer-1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
