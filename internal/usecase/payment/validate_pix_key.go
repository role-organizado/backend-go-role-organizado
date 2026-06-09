package payment

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// Pre-compiled regular expressions for PIX key format validation.
var (
	reEmail   = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	rePhone   = regexp.MustCompile(`^\+55\d{10,11}$`)
	reUUID    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// validatePixKeyUseCase implements portin.ValidatePixKeyUseCase with local format validation.
type validatePixKeyUseCase struct{}

// NewValidatePixKey creates a new ValidatePixKeyUseCase (local format validation only).
func NewValidatePixKey() portin.ValidatePixKeyUseCase {
	return &validatePixKeyUseCase{}
}

// Execute validates the PIX key format locally and returns the detected key type.
// The userID parameter is accepted for audit/rate-limiting purposes but not used
// in the current local-only implementation.
func (uc *validatePixKeyUseCase) Execute(ctx context.Context, userID, pixKey string) (*portin.ValidatePixKeyResult, error) {
	key := strings.TrimSpace(pixKey)
	if key == "" {
		return &portin.ValidatePixKeyResult{
			Valid:   false,
			Key:     pixKey,
			KeyType: "",
		}, nil
	}

	keyType, valid, msg := detectAndValidate(key)
	_ = msg // msg is informational; errors return valid=false

	return &portin.ValidatePixKeyResult{
		Valid:   valid,
		Key:     key,
		KeyType: keyType,
	}, nil
}

// detectAndValidate infers the PIX key type from its format and validates it.
// Returns (keyType, valid, humanMessage).
func detectAndValidate(key string) (string, bool, string) {
	digits := onlyDigits(key)

	switch {
	case reUUID.MatchString(key):
		return "CHAVE_ALEATORIA", true, "chave aleatória UUID válida"

	case reEmail.MatchString(key):
		return "EMAIL", true, "endereço de e-mail válido"

	case strings.HasPrefix(key, "+"):
		if rePhone.MatchString(key) {
			return "TELEFONE", true, "número de telefone brasileiro válido"
		}
		return "TELEFONE", false, "número de telefone inválido: deve seguir o formato +55XXXXXXXXXXX (10 ou 11 dígitos após o DDD)"

	case len(digits) == 11:
		return "CPF", true, "CPF válido"

	case len(digits) == 14:
		return "CNPJ", true, "CNPJ válido"

	default:
		return "", false, "formato de chave PIX não reconhecido: " + key
	}
}

// onlyDigits strips all non-digit characters from s.
func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
