package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/auth"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	ucauth "github.com/role-organizado/backend-go-role-organizado/internal/usecase/auth"
)

// boolPtr is a small helper used by the tests to keep the call sites compact.
func boolPtr(b bool) *bool { return &b }

// TestUpdateUsuario_AiMemoryOptOut_SetTrue verifies that when AiMemoryOptOut is
// supplied (non-nil) the value is applied to the loaded usuario before persistence.
func TestUpdateUsuario_AiMemoryOptOut_SetTrue(t *testing.T) {
	repo := new(mockUsuarioRepo)
	existing := &domain.Usuario{
		ID:    "u1",
		Nome:  "Alice",
		Email: "a@x.com",
		// AiMemoryOptOut starts nil (default — not opted-out).
	}

	repo.On("FindByID", mock.Anything, "u1").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(u *domain.Usuario) bool {
		return u.AiMemoryOptOut != nil && *u.AiMemoryOptOut
	})).Return(&domain.Usuario{
		ID:             "u1",
		Nome:           "Alice",
		Email:          "a@x.com",
		AiMemoryOptOut: boolPtr(true),
	}, nil)

	uc := ucauth.NewUpdateUsuario(repo)
	got, err := uc.Execute(context.Background(), "u1", portin.UpdateUsuarioInput{
		AiMemoryOptOut: boolPtr(true),
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.AiMemoryOptOut)
	assert.True(t, *got.AiMemoryOptOut)
	repo.AssertExpectations(t)
}

// TestUpdateUsuario_AiMemoryOptOut_NilLeavesUntouched verifies that an absent
// AiMemoryOptOut input preserves whatever value was previously stored — true
// fetch-then-patch semantics, matching Java's partialUpdate behaviour.
func TestUpdateUsuario_AiMemoryOptOut_NilLeavesUntouched(t *testing.T) {
	repo := new(mockUsuarioRepo)
	existing := &domain.Usuario{
		ID:             "u1",
		Nome:           "Alice",
		Email:          "a@x.com",
		AiMemoryOptOut: boolPtr(true), // previously set to true
	}

	repo.On("FindByID", mock.Anything, "u1").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(u *domain.Usuario) bool {
		// Field must be preserved (true) — Nome change does not touch it.
		return u.AiMemoryOptOut != nil && *u.AiMemoryOptOut && u.Nome == "Alice 2"
	})).Return(&domain.Usuario{
		ID:             "u1",
		Nome:           "Alice 2",
		Email:          "a@x.com",
		AiMemoryOptOut: boolPtr(true),
	}, nil)

	uc := ucauth.NewUpdateUsuario(repo)
	got, err := uc.Execute(context.Background(), "u1", portin.UpdateUsuarioInput{
		Nome: "Alice 2", // only Nome — AiMemoryOptOut intentionally omitted
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.AiMemoryOptOut)
	assert.True(t, *got.AiMemoryOptOut)
	repo.AssertExpectations(t)
}

// TestUpdateUsuario_AiMemoryOptOut_ExplicitFalse verifies that an explicit
// false value can override a previously-true preference (the user revoked the
// opt-out and consents to AI memory injection again).
func TestUpdateUsuario_AiMemoryOptOut_ExplicitFalse(t *testing.T) {
	repo := new(mockUsuarioRepo)
	existing := &domain.Usuario{
		ID:             "u1",
		Nome:           "Alice",
		Email:          "a@x.com",
		AiMemoryOptOut: boolPtr(true),
	}

	repo.On("FindByID", mock.Anything, "u1").Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(u *domain.Usuario) bool {
		return u.AiMemoryOptOut != nil && !*u.AiMemoryOptOut
	})).Return(&domain.Usuario{
		ID:             "u1",
		Nome:           "Alice",
		Email:          "a@x.com",
		AiMemoryOptOut: boolPtr(false),
	}, nil)

	uc := ucauth.NewUpdateUsuario(repo)
	got, err := uc.Execute(context.Background(), "u1", portin.UpdateUsuarioInput{
		AiMemoryOptOut: boolPtr(false),
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.AiMemoryOptOut)
	assert.False(t, *got.AiMemoryOptOut)
	repo.AssertExpectations(t)
}
