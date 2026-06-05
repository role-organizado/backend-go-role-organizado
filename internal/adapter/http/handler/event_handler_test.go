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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/event"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// ---- mocks ----

type mockCreateEventoUC struct{ mock.Mock }

func (m *mockCreateEventoUC) Execute(ctx context.Context, in portin.CreateEventoInput) (*domain.Evento, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

type mockGetEventoUC struct{ mock.Mock }

func (m *mockGetEventoUC) Execute(ctx context.Context, id string) (*domain.Evento, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

type mockListEventosUC struct{ mock.Mock }

func (m *mockListEventosUC) Execute(ctx context.Context, usuarioID *string, page, pageSize int) ([]domain.Evento, int64, error) {
	args := m.Called(ctx, usuarioID, page, pageSize)
	return args.Get(0).([]domain.Evento), int64(args.Int(1)), args.Error(2)
}

type mockUpdateEventoUC struct{ mock.Mock }

func (m *mockUpdateEventoUC) Execute(ctx context.Context, id, requesterID string, in portin.UpdateEventoInput) (*domain.Evento, error) {
	args := m.Called(ctx, id, requesterID, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Evento), args.Error(1)
}

type mockDeleteEventoUC struct{ mock.Mock }

func (m *mockDeleteEventoUC) Execute(ctx context.Context, id, requesterID string) error {
	args := m.Called(ctx, id, requesterID)
	return args.Error(0)
}

type mockListEventosByUsuarioUC struct{ mock.Mock }

func (m *mockListEventosByUsuarioUC) Execute(ctx context.Context, in portin.ListEventosByUsuarioInput) (portout.EventosCursorPage, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(portout.EventosCursorPage), args.Error(1)
}

type mockAddConvidadosUC struct{ mock.Mock }

func (m *mockAddConvidadosUC) Execute(ctx context.Context, in portin.AddConvidadosInput) error {
	args := m.Called(ctx, in)
	return args.Error(0)
}

// ---- helpers ----

func newTestEventHandler(
	createUC portin.CreateEventoUseCase,
	getUC portin.GetEventoUseCase,
	listUC portin.ListEventosUseCase,
	updateUC portin.UpdateEventoUseCase,
	deleteUC portin.DeleteEventoUseCase,
	listByUsuarioUC portin.ListEventosByUsuarioUseCase,
	addConvidadosUC portin.AddConvidadosUseCase,
) *handler.EventHandler {
	return handler.NewEventHandler(createUC, getUC, listUC, updateUC, deleteUC, listByUsuarioUC, addConvidadosUC)
}

func sampleEventoForHandler(id, userID string) *domain.Evento {
	return &domain.Evento{
		ID:        id,
		UsuarioID: userID,
		Nome:      "Festa",
		Tipo:      "festa",
		Data:      time.Now().Add(24 * time.Hour),
		Status:    domain.EventoStatusPublicado,
		CriadoEm: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---- GET /api/eventos/v1/eventos/usuario/{userId} tests ----

func TestEventListByUsuarioV1_Success(t *testing.T) {
	listByUC := new(mockListEventosByUsuarioUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), listByUC, new(mockAddConvidadosUC),
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	cursor := "dGVzdA=="
	page := portout.EventosCursorPage{
		Eventos:     []domain.Evento{*sampleEventoForHandler("evt-1", "user-1")},
		Total:       1,
		NextCursor:  &cursor,
		HasNextPage: false,
		Limit:       20,
	}
	listByUC.On("Execute", mock.Anything, mock.MatchedBy(func(in portin.ListEventosByUsuarioInput) bool {
		return in.UsuarioID == "user-1" && in.RequesterID == "user-1"
	})).Return(page, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/eventos/v1/eventos/usuario/user-1", nil)
	// inject authenticated user ID using the middleware helper
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-1"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp, "eventos")
	assert.Contains(t, resp, "total")
	assert.Contains(t, resp, "nextCursor")
	assert.Contains(t, resp, "hasNextPage")
	assert.Contains(t, resp, "limit")
	listByUC.AssertExpectations(t)
}

func TestEventListByUsuarioV1_Unauthorized(t *testing.T) {
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		new(mockAddConvidadosUC),
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	// No user ID in context → 401
	req := httptest.NewRequest(http.MethodGet, "/api/eventos/v1/eventos/usuario/user-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestEventListByUsuarioV1_ResponseShape(t *testing.T) {
	listByUC := new(mockListEventosByUsuarioUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), listByUC, new(mockAddConvidadosUC),
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	// Empty page, no cursor
	page := portout.EventosCursorPage{
		Eventos:     []domain.Evento{},
		Total:       0,
		NextCursor:  nil,
		HasNextPage: false,
		Limit:       20,
	}
	listByUC.On("Execute", mock.Anything, mock.Anything).Return(page, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/eventos/v1/eventos/usuario/user-2", nil)
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-2"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// All required fields present
	_, hasEventos := resp["eventos"]
	_, hasTotal := resp["total"]
	_, hasNextCursor := resp["nextCursor"]
	_, hasHasNextPage := resp["hasNextPage"]
	_, hasLimit := resp["limit"]
	assert.True(t, hasEventos)
	assert.True(t, hasTotal)
	assert.True(t, hasNextCursor)
	assert.True(t, hasHasNextPage)
	assert.True(t, hasLimit)
}

// ---- POST /api/eventos/v1/eventos/{eventoId}/convidados tests ----

func TestEventAddConvidados_Success(t *testing.T) {
	addConvUC := new(mockAddConvidadosUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		addConvUC,
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	addConvUC.On("Execute", mock.Anything, mock.MatchedBy(func(in portin.AddConvidadosInput) bool {
		return in.EventoID == "evt-123" &&
			in.UsuarioID == "user-1" &&
			len(in.Convidados) == 1 &&
			in.Convidados[0].Telefone == "+5511999990001" &&
			in.Convidados[0].Nome == "Fulano"
	})).Return(nil)

	body := `{"convidados":[{"telefone":"+5511999990001","nome":"Fulano"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-123/convidados", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Usuario-Id", "user-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "convidados adicionados com sucesso", resp["message"])
	assert.Equal(t, "evt-123", resp["eventoId"])
	assert.Equal(t, float64(1), resp["quantidade"])
	addConvUC.AssertExpectations(t)
}

func TestEventAddConvidados_MultipleGuests(t *testing.T) {
	addConvUC := new(mockAddConvidadosUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		addConvUC,
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	addConvUC.On("Execute", mock.Anything, mock.MatchedBy(func(in portin.AddConvidadosInput) bool {
		return len(in.Convidados) == 2
	})).Return(nil)

	body := `{"convidados":[{"telefone":"+5511999990001","nome":"Ana"},{"telefone":"+5511999990002","nome":"Bob"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-456/convidados", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Usuario-Id", "user-2")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	addConvUC.AssertExpectations(t)
}

func TestEventAddConvidados_MissingXUsuarioId(t *testing.T) {
	addConvUC := new(mockAddConvidadosUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		addConvUC,
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	body := `{"convidados":[{"telefone":"+5511999990001","nome":"Fulano"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-123/convidados", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// X-Usuario-Id header intentionally omitted

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "X-Usuario-Id")
}

func TestEventAddConvidados_EmptyList(t *testing.T) {
	addConvUC := new(mockAddConvidadosUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		addConvUC,
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	body := `{"convidados":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-123/convidados", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Usuario-Id", "user-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEventAddConvidados_EventNotFound(t *testing.T) {
	addConvUC := new(mockAddConvidadosUC)
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		addConvUC,
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	addConvUC.On("Execute", mock.Anything, mock.Anything).
		Return(apierr.NotFound("evento", "evt-999"))

	body := `{"convidados":[{"telefone":"+5511999990001","nome":"Fulano"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-999/convidados", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Usuario-Id", "user-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEventAddConvidados_InvalidBody(t *testing.T) {
	h := newTestEventHandler(
		new(mockCreateEventoUC), new(mockGetEventoUC), new(mockListEventosUC),
		new(mockUpdateEventoUC), new(mockDeleteEventoUC), new(mockListEventosByUsuarioUC),
		new(mockAddConvidadosUC),
	)

	r := chi.NewRouter()
	h.RegisterEventRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/eventos/v1/eventos/evt-123/convidados", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Usuario-Id", "user-1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
