package payment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ucpayment "github.com/role-organizado/backend-go-role-organizado/internal/usecase/payment"
)

func TestValidatePix_ValidCPF(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "12345678901")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "CPF", res.KeyType)
}

func TestValidatePix_InvalidCPF_TooShort(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	// 10 digits (not CPF or CNPJ)
	res, err := uc.Execute(context.Background(), "usr-1", "1234567890")
	require.NoError(t, err)
	assert.False(t, res.Valid)
}

func TestValidatePix_ValidCNPJ(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "12345678000195")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "CNPJ", res.KeyType)
}

func TestValidatePix_ValidEmail(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "user@example.com")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "EMAIL", res.KeyType)
}

func TestValidatePix_InvalidEmail(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "not-an-email")
	require.NoError(t, err)
	assert.False(t, res.Valid)
}

func TestValidatePix_ValidPhone_11Digits(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	// +55 + 11 digits (DDD + 9-digit mobile)
	res, err := uc.Execute(context.Background(), "usr-1", "+5511999999999")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "TELEFONE", res.KeyType)
}

func TestValidatePix_ValidPhone_10Digits(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	// +55 + 10 digits (DDD + 8-digit landline)
	res, err := uc.Execute(context.Background(), "usr-1", "+551133334444")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "TELEFONE", res.KeyType)
}

func TestValidatePix_InvalidPhone_NoCountryCode(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "+1199999999")
	require.NoError(t, err)
	assert.False(t, res.Valid)
	assert.Equal(t, "TELEFONE", res.KeyType)
}

func TestValidatePix_InvalidPhone_MissingPlus(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	// 11 digits but no + prefix → detected as CPF
	res, err := uc.Execute(context.Background(), "usr-1", "55119999999999")
	require.NoError(t, err)
	// 14 digits with no + → looks like CNPJ
	assert.Equal(t, "CNPJ", res.KeyType)
}

func TestValidatePix_ValidChaveAleatoria(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.True(t, res.Valid)
	assert.Equal(t, "CHAVE_ALEATORIA", res.KeyType)
}

func TestValidatePix_InvalidUUID_MissingHyphens(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "550e8400e29b41d4a716446655440000")
	require.NoError(t, err)
	// 32 hex chars without hyphens — not a UUID, not CPF/CNPJ → invalid
	assert.False(t, res.Valid)
}

func TestValidatePix_EmptyKey_ReturnsFalse(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	res, err := uc.Execute(context.Background(), "usr-1", "")
	require.NoError(t, err)
	assert.False(t, res.Valid)
}

func TestValidatePix_TableDriven(t *testing.T) {
	uc := ucpayment.NewValidatePixKey()
	tests := []struct {
		name        string
		key         string
		wantValid   bool
		wantKeyType string
	}{
		{"cpf_11digits", "12345678901", true, "CPF"},
		{"cnpj_14digits", "12345678000195", true, "CNPJ"},
		{"email_valid", "test@domain.org", true, "EMAIL"},
		{"phone_mobile", "+5511987654321", true, "TELEFONE"},
		{"uuid_lower", "6ba7b810-9dad-11d1-80b4-00c04fd430c8", true, "CHAVE_ALEATORIA"},
		{"uuid_upper", "6BA7B810-9DAD-11D1-80B4-00C04FD430C8", true, "CHAVE_ALEATORIA"},
		{"empty", "", false, ""},
		{"short_digits", "1234567890", false, ""},
		{"invalid_phone", "+5511", false, "TELEFONE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := uc.Execute(context.Background(), "usr-1", tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.wantValid, res.Valid, "key: %q", tc.key)
			assert.Equal(t, tc.wantKeyType, res.KeyType, "key: %q", tc.key)
		})
	}
}
