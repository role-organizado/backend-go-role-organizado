package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/http/handler"
)

func TestCardapioHandler_ListCardapios_NilMongo_ReturnsEmptyArray(t *testing.T) {
	// nil mongo → handler should return 200 with empty JSON array
	h := handler.NewCardapioHandler(nil)
	r := chi.NewRouter()
	h.RegisterCardapioRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/api/cardapios", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result []interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestCardapioHandler_ListCardapios_MethodNotAllowed(t *testing.T) {
	h := handler.NewCardapioHandler(nil)
	r := chi.NewRouter()
	h.RegisterCardapioRoutes(r)

	// POST /api/cardapios should 405
	req := httptest.NewRequest(http.MethodPost, "/api/cardapios", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
