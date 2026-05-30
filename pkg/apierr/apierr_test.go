package apierr_test

import (
	"errors"
	"testing"

	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotFound(t *testing.T) {
	err := apierr.NotFound("evento", "abc123")
	require.NotNil(t, err)
	assert.Equal(t, 404, err.Status)
	assert.Equal(t, apierr.CodeNotFound, err.Code)
	assert.Contains(t, err.Message, "abc123")
	assert.Contains(t, err.Error(), "NOT_FOUND")
}

func TestUnauthorized(t *testing.T) {
	err := apierr.Unauthorized("token inválido")
	assert.Equal(t, 401, err.Status)
	assert.Equal(t, apierr.CodeUnauthorized, err.Code)
}

func TestForbidden(t *testing.T) {
	err := apierr.Forbidden("acesso negado")
	assert.Equal(t, 403, err.Status)
}

func TestBadRequest(t *testing.T) {
	err := apierr.BadRequest("campo obrigatório ausente")
	assert.Equal(t, 400, err.Status)
}

func TestConflict(t *testing.T) {
	err := apierr.Conflict("email já cadastrado")
	assert.Equal(t, 409, err.Status)
}

func TestInternal(t *testing.T) {
	err := apierr.Internal("db timeout")
	assert.Equal(t, 500, err.Status)
}

func TestFrom_WithAPIError(t *testing.T) {
	original := apierr.NotFound("evento", "xyz")
	wrapped := apierr.From(original)
	assert.Equal(t, original, wrapped)
}

func TestFrom_WithGenericError(t *testing.T) {
	generic := errors.New("conexão recusada")
	result := apierr.From(generic)
	assert.Equal(t, 500, result.Status)
	assert.Contains(t, result.Message, "conexão recusada")
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"not found error", apierr.NotFound("x", "1"), true},
		{"unauthorized error", apierr.Unauthorized("x"), false},
		{"generic error", errors.New("any"), false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, apierr.IsNotFound(tt.err))
		})
	}
}

func TestBadRequestWithDetails(t *testing.T) {
	details := map[string]string{"nome": "obrigatório"}
	err := apierr.BadRequestWithDetails("validação falhou", details)
	assert.Equal(t, 400, err.Status)
	assert.NotNil(t, err.Details)
}
