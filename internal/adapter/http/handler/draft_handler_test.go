package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mock use cases ----

type mockCreateDraftUC struct{ mock.Mock }

func (m *mockCreateDraftUC) Execute(ctx context.Context, usuarioID string) (*domain.EventoDraft, error) {
	args := m.Called(ctx, usuarioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

type mockGetDraftUC struct{ mock.Mock }

func (m *mockGetDraftUC) Execute(ctx context.Context, id, requesterID string) (*domain.EventoDraft, error) {
	args := m.Called(ctx, id, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

type mockListDraftsUC struct{ mock.Mock }

func (m *mockListDraftsUC) Execute(ctx context.Context, usuarioID string) ([]domain.EventoDraft, error) {
	args := m.Called(ctx, usuarioID)
	return args.Get(0).([]domain.EventoDraft), args.Error(1)
}

type mockUpdateDraftUC struct{ mock.Mock }

func (m *mockUpdateDraftUC) Execute(ctx context.Context, id, requesterID string, in portin.UpsertDraftInput) (*domain.EventoDraft, error) {
	args := m.Called(ctx, id, requesterID, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.EventoDraft), args.Error(1)
}

type mockDeleteDraftUC struct{ mock.Mock }

func (m *mockDeleteDraftUC) Execute(ctx context.Context, id, requesterID string) error {
	args := m.Called(ctx, id, requesterID)
	return args.Error(0)
}

type mockPublishDraftUC struct{ mock.Mock }

func (m *mockPublishDraftUC) Execute(ctx context.Context, draftID, requesterID string) (*domain.Evento, error) {
	args := m.Called(ctx, draftID, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

type mockValidateDraftUC struct{ mock.Mock }

func (m *mockValidateDraftUC) Execute(ctx context.Context, id, requesterID string) ([]portin.ValidationResult, error) {
	args := m.Called(ctx, id, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]portin.ValidationResult), args.Error(1)
}

// ---- helpers ----

func newTestDraftHandler(
	createUC portin.CreateDraftUseCase,
	getUC portin.GetDraftUseCase,
	listUC portin.ListDraftsUseCase,
	updateUC portin.UpdateDraftUseCase,
	deleteUC portin.DeleteDraftUseCase,
	publishUC portin.PublishDraftUseCase,
	validateUC portin.ValidateDraftUseCase,
) *chi.Mux {
	h := handler.NewDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)
	r := chi.NewRouter()
	h.RegisterDraftRoutes(r)
	return r
}

// withUser wraps a request with the given userID injected into the context.
func withUser(r *http.Request, userID string) *http.Request {
	ctx := middleware.WithUserIDContext(r.Context(), userID)
	return r.WithContext(ctx)
}

// ---- ValidateDraft handler tests ----

func TestDraftHandler_ValidateDraft_AllValid(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	results := []portin.ValidationResult{
		{Field: "nome", Valid: true},
		{Field: "tipo", Valid: true},
		{Field: "data", Valid: true},
		{Field: "local", Valid: true},
	}
	validateUC.On("Execute", mock.Anything, "draft-1", "user-1").Return(results, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/validate", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body []portin.ValidationResult
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 4)
	for _, v := range body {
		assert.True(t, v.Valid)
	}
	validateUC.AssertExpectations(t)
}

func TestDraftHandler_ValidateDraft_InvalidFields(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	results := []portin.ValidationResult{
		{Field: "nome", Valid: false, Message: "Título é obrigatório"},
		{Field: "tipo", Valid: true},
		{Field: "data", Valid: false, Message: "Data do evento é obrigatória"},
		{Field: "local", Valid: true},
	}
	validateUC.On("Execute", mock.Anything, "draft-1", "user-1").Return(results, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/validate", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "UNPROCESSABLE_ENTITY", body["code"])
	assert.NotNil(t, body["validations"])
	validateUC.AssertExpectations(t)
}

func TestDraftHandler_ValidateDraft_Unauthenticated(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	// No user ID in context
	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/validate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDraftHandler_ValidateDraft_Forbidden(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	validateUC.On("Execute", mock.Anything, "draft-1", "other-user").Return(
		nil, apierr.Forbidden("acesso negado"),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/validate", nil)
	req = withUser(req, "other-user")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	validateUC.AssertExpectations(t)
}

func TestDraftHandler_ValidateDraft_EventosDraftAlias(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	results := []portin.ValidationResult{
		{Field: "nome", Valid: true},
		{Field: "tipo", Valid: true},
		{Field: "data", Valid: true},
		{Field: "local", Valid: true},
	}
	validateUC.On("Execute", mock.Anything, "draft-1", "user-1").Return(results, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos-draft/draft-1/validate", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	validateUC.AssertExpectations(t)
}

// ---- PublicarDraft handler tests ----

func TestDraftHandler_PublicarDraft_Success(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	evt := &domain.Evento{
		ID:        "evt-123",
		UsuarioID: "user-1",
		Nome:      "Festa",
		CriadoEm: time.Now(),
		UpdatedAt: time.Now(),
	}
	publishUC.On("Execute", mock.Anything, "draft-1", "user-1").Return(evt, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/publicar", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "evt-123", body["eventoId"])
	publishUC.AssertExpectations(t)
}

func TestDraftHandler_PublicarDraft_Unauthenticated(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/draft-1/publicar", nil)
	// no user in context
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDraftHandler_PublicarDraft_EventosDraftAlias(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	evt := &domain.Evento{
		ID:        "evt-456",
		UsuarioID: "user-1",
		Nome:      "Churrasco",
		CriadoEm: time.Now(),
		UpdatedAt: time.Now(),
	}
	publishUC.On("Execute", mock.Anything, "draft-2", "user-1").Return(evt, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/eventos-draft/draft-2/publicar", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "evt-456", body["eventoId"])
	publishUC.AssertExpectations(t)
}

func TestDraftHandler_PublicarDraft_NotFound(t *testing.T) {
	createUC := new(mockCreateDraftUC)
	getUC := new(mockGetDraftUC)
	listUC := new(mockListDraftsUC)
	updateUC := new(mockUpdateDraftUC)
	deleteUC := new(mockDeleteDraftUC)
	publishUC := new(mockPublishDraftUC)
	validateUC := new(mockValidateDraftUC)

	r := newTestDraftHandler(createUC, getUC, listUC, updateUC, deleteUC, publishUC, validateUC)

	publishUC.On("Execute", mock.Anything, "nonexistent", "user-1").Return(
		nil, apierr.NotFound("draft", "nonexistent"),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/drafts/nonexistent/publicar", nil)
	req = withUser(req, "user-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	publishUC.AssertExpectations(t)
}
