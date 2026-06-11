package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/middleware"
	outbounddomain "github.com/role-organizado/backend-go-role-organizado/internal/domain/outbound"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// ─── Minimal use-case stubs used only by the handler routing tests. ──────────

type stubCreateOutboundUC struct{}

func (stubCreateOutboundUC) Execute(ctx context.Context, in portin.CreateOutboundRequestInput) (*outbounddomain.OutboundRequest, error) {
	return &outbounddomain.OutboundRequest{ID: "x", EventID: in.EventID, RequesterUserID: in.UserID}, nil
}

type stubListOutboundUC struct{}

func (stubListOutboundUC) ListByEvent(ctx context.Context, userID, eventID string) ([]outbounddomain.OutboundRequest, error) {
	return []outbounddomain.OutboundRequest{}, nil
}
func (stubListOutboundUC) ListByEventAndStatus(ctx context.Context, userID, eventID string, status outbounddomain.OutboundStatus) ([]outbounddomain.OutboundRequest, error) {
	return []outbounddomain.OutboundRequest{}, nil
}
func (stubListOutboundUC) ListByEventAndType(ctx context.Context, userID, eventID string, t outbounddomain.OutboundType) ([]outbounddomain.OutboundRequest, error) {
	return []outbounddomain.OutboundRequest{}, nil
}
func (stubListOutboundUC) ListPendingByEvent(ctx context.Context, userID, eventID string) ([]outbounddomain.OutboundRequest, error) {
	return []outbounddomain.OutboundRequest{}, nil
}
func (stubListOutboundUC) CountPendingByEvent(ctx context.Context, userID, eventID string) (int64, error) {
	return 0, nil
}
func (stubListOutboundUC) ListMyRequests(ctx context.Context, userID string) ([]outbounddomain.OutboundRequest, error) {
	return []outbounddomain.OutboundRequest{}, nil
}

type stubGetOutboundUC struct{}

func (stubGetOutboundUC) Execute(ctx context.Context, userID, requestID string) (*outbounddomain.OutboundRequest, error) {
	return nil, nil
}
func (stubGetOutboundUC) ExecuteByEvent(ctx context.Context, userID, requestID, eventID string) (*outbounddomain.OutboundRequest, error) {
	return nil, nil
}

type stubGetDetailsUC struct{}

func (stubGetDetailsUC) Execute(ctx context.Context, userID, requestID string) (*portin.OutboundDetailsResult, error) {
	return &portin.OutboundDetailsResult{}, nil
}

type stubApproveUC struct{}

func (stubApproveUC) Execute(ctx context.Context, in portin.ApproveOutboundRequestInput) (*outbounddomain.OutboundRequest, error) {
	return nil, nil
}

type stubRejectUC struct{}

func (stubRejectUC) Execute(ctx context.Context, in portin.RejectOutboundRequestInput) (*outbounddomain.OutboundRequest, error) {
	return nil, nil
}

type stubCancelUC struct{}

func (stubCancelUC) Execute(ctx context.Context, in portin.CancelOutboundRequestInput) (*outbounddomain.OutboundRequest, error) {
	return nil, nil
}

type stubVoteUC struct{}

func (stubVoteUC) Execute(ctx context.Context, in portin.VoteOnOutboundRequestInput) (*portin.VoteResult, error) {
	return &portin.VoteResult{}, nil
}

func newStubOutboundHandler() *handler.OutboundRequestHandler {
	return handler.NewOutboundRequestHandler(
		stubCreateOutboundUC{},
		stubListOutboundUC{},
		stubGetOutboundUC{},
		stubGetDetailsUC{},
		stubApproveUC{},
		stubRejectUC{},
		stubCancelUC{},
		stubVoteUC{},
	)
}

func TestOutboundRequestHandler_MyRequests_NoAuth_Returns401(t *testing.T) {
	// No JWT context → should return 401 Unauthorized.
	h := newStubOutboundHandler()
	r := chi.NewRouter()
	h.RegisterOutboundRequestRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/outbound-requests/my-requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOutboundRequestHandler_MyRequests_WithAuth_ReturnsEmptyArray(t *testing.T) {
	// Authenticated user → 200 with empty JSON array from the stub UC.
	h := newStubOutboundHandler()
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := middleware.ContextWithUserID(req.Context(), "user-test-123")
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	h.RegisterOutboundRequestRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/outbound-requests/my-requests", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result []any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Empty(t, result)
}
