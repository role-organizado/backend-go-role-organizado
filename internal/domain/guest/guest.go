// Package guest holds the Guest, BiometricCredential and BiometricChallenge entities.
//
// Guests are lightweight contact records used to invite people to events before they
// become full Usuarios (see VincularGuestAUsuarioUseCase on the Java side). Once a
// Guest evolves into a Usuario, EvoluidoParaUsuarioID is set and remains immutable.
//
// Java parity: matches the Guest / BiometricCredential / BiometricChallenge entities
// of the Spring Boot backend, including collection names and BSON field names.
package guest

import (
	"strings"
	"time"
	"unicode"
)

// Guest is a lightweight contact (telefone OR email) used pre-account-creation.
// Collection: guests
type Guest struct {
	ID                    string
	Nome                  string
	Telefone              string // E.164 normalized (+5511999999999)
	Email                 string
	CriadoEm              time.Time
	AtualizadoEm          time.Time
	EvoluidoParaUsuarioID string
	EvoluidoEm            *time.Time
}

// IsEvoluido reports whether the guest has been linked to a Usuario already.
func (g *Guest) IsEvoluido() bool {
	return g.EvoluidoParaUsuarioID != ""
}

// IdentificadorUnico returns the first non-empty unique identifier (telefone > email).
func (g *Guest) IdentificadorUnico() string {
	if g.Telefone != "" {
		return g.Telefone
	}
	return g.Email
}

// NormalizeTelefone returns the input trimmed and stripped of all non-digit characters
// except a single leading '+'. Empty input returns empty string.
//
// Matches Java's E.164 regex ^\+?[1-9]\d{1,14}$ — we keep a leading + if present
// and strip whitespace, parentheses, hyphens, dots, etc.
func NormalizeTelefone(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	leadingPlus := false
	if s[0] == '+' {
		leadingPlus = true
		s = s[1:]
	}
	var b strings.Builder
	if leadingPlus {
		b.WriteByte('+')
	}
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsValidE164 returns true when phone matches Java's ^\+?[1-9]\d{1,14}$ pattern.
func IsValidE164(phone string) bool {
	s := phone
	if s == "" {
		return false
	}
	if s[0] == '+' {
		s = s[1:]
	}
	if len(s) < 2 || len(s) > 15 {
		return false
	}
	if s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
