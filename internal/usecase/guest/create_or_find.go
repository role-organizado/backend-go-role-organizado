// Package guest holds the Guest + Biometric Auth use cases.
//
// Java parity sources:
//   - application/usecase/guest/CriarOuBuscarGuestUseCase.java
//   - application/usecase/BiometricAuthUseCase.java
//
// Hexagonal: all use cases depend on output ports (out.GuestRepository,
// out.BiometricCredentialRepository, out.BiometricChallengeRepository,
// out.UsuarioRepository, out.RefreshTokenRepository) and on pkg/jwt.Service.
package guest

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/guest"
	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
	portout "github.com/role-organizado/backend-go-role-organizado/internal/port/out"
	"github.com/role-organizado/backend-go-role-organizado/pkg/apierr"
)

// CreateOrFind implements portin.CreateOrFindGuestUseCase.
type CreateOrFind struct {
	guests portout.GuestRepository
}

// NewCreateOrFindGuest builds the use case.
func NewCreateOrFindGuest(guests portout.GuestRepository) *CreateOrFind {
	return &CreateOrFind{guests: guests}
}

// Execute reproduces the Java create-or-find pattern:
//  1. validate nome (>=2 chars) and at least one of (telefone, email) non-empty,
//  2. normalize telefone to E.164,
//  3. look up by phone (priority) OR email,
//  4. update nome if the existing record differs, OR insert a new Guest with a fresh UUID.
func (uc *CreateOrFind) Execute(ctx context.Context, in portin.CreateOrFindGuestInput) (*domain.Guest, error) {
	nome := strings.TrimSpace(in.Nome)
	if len(nome) < 2 {
		return nil, apierr.BadRequest("nome deve ter no mínimo 2 caracteres")
	}
	telefone := domain.NormalizeTelefone(in.Telefone)
	email := strings.TrimSpace(in.Email)
	if telefone == "" && email == "" {
		return nil, apierr.BadRequest("telefone ou email é obrigatório")
	}
	if telefone != "" && !domain.IsValidE164(telefone) {
		return nil, apierr.BadRequest("telefone inválido (formato E.164 esperado)")
	}

	existing, err := uc.guests.FindByTelefoneOrEmail(ctx, telefone, email)
	if err != nil && !apierr.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		if existing.Nome != nome {
			existing.Nome = nome
			existing.AtualizadoEm = time.Now().UTC()
			updated, uerr := uc.guests.Update(ctx, existing)
			if uerr != nil {
				return nil, uerr
			}
			return updated, nil
		}
		return existing, nil
	}

	now := time.Now().UTC()
	g := &domain.Guest{
		ID:           uuid.New().String(),
		Nome:         nome,
		Telefone:     telefone,
		Email:        email,
		CriadoEm:     now,
		AtualizadoEm: now,
	}
	slog.InfoContext(ctx, "criando novo guest", "guestId", g.ID, "telefone", telefone, "email", email)
	return uc.guests.Save(ctx, g)
}
