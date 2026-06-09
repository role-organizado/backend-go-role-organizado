// Package finance tests the domain functions for CalculateProgress and ValidatePixKey.
// These functions contain pure business logic with no external dependencies.
package finance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// CalculateProgress
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateProgress(t *testing.T) {
	tests := []struct {
		name      string
		collected int64
		goal      int64
		want      float64
	}{
		{
			name:      "goal zero retorna 0 (sem divisão por zero)",
			collected: 1000,
			goal:      0,
			want:      0,
		},
		{
			name:      "collected zero retorna 0",
			collected: 0,
			goal:      10000,
			want:      0,
		},
		{
			name:      "50% retorna 50.0000",
			collected: 5000,
			goal:      10000,
			want:      50.0,
		},
		{
			name:      "60% retorna 60.0000 (seed do parity test)",
			collected: 30_000,
			goal:      50_000,
			want:      60.0,
		},
		{
			name:      "100% retorna 100.0000",
			collected: 10000,
			goal:      10000,
			want:      100.0,
		},
		{
			name:      "HALF_UP — 33.3367 (4 casas decimais)",
			// 10001/30000 * 100 = 33.33666...
			// math.Round(33.33666... * 10000) / 10000
			// = math.Round(333366.67) / 10000 = 333367 / 10000 = 33.3367
			collected: 10001,
			goal:      30000,
			want:      33.3367,
		},
		{
			name:      "HALF_UP — arredondamento para baixo (33.3333)",
			// 10000/30000 * 100 = 33.33333...
			// math.Round(333333.33...) / 10000 = 333333 / 10000 = 33.3333
			collected: 10000,
			goal:      30000,
			want:      33.3333,
		},
		{
			name:      "progress > 100% permitido (collected > goal)",
			collected: 12000,
			goal:      10000,
			want:      120.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateProgress(tt.collected, tt.goal)
			assert.InDelta(t, tt.want, got, 0.00001,
				"CalculateProgress(%d, %d) should be %.5f, got %.5f",
				tt.collected, tt.goal, tt.want, got)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidatePixKey
// ─────────────────────────────────────────────────────────────────────────────

func TestValidatePixKey(t *testing.T) {
	tests := []struct {
		name    string
		pixType string
		key     string
		wantErr bool
		errMsg  string
	}{
		// ── CPF ──────────────────────────────────────────────────────────
		{
			name:    "CPF válido com 11 dígitos",
			pixType: "CPF",
			key:     "12345678901",
			wantErr: false,
		},
		{
			name:    "CPF inválido — menos de 11 dígitos",
			pixType: "CPF",
			key:     "1234567890",
			wantErr: true,
			errMsg:  "11 dígitos",
		},
		{
			name:    "CPF inválido — mais de 11 dígitos",
			pixType: "CPF",
			key:     "123456789012",
			wantErr: true,
			errMsg:  "11 dígitos",
		},
		{
			name:    "CPF inválido — contém letra",
			pixType: "CPF",
			key:     "1234567890a",
			wantErr: true,
			errMsg:  "apenas dígitos",
		},
		{
			name:    "CPF case-insensitive — tipo em minúsculas",
			pixType: "cpf",
			key:     "12345678901",
			wantErr: false,
		},
		// ── CNPJ ─────────────────────────────────────────────────────────
		{
			name:    "CNPJ válido com 14 dígitos",
			pixType: "CNPJ",
			key:     "12345678000190",
			wantErr: false,
		},
		{
			name:    "CNPJ inválido — menos de 14 dígitos",
			pixType: "CNPJ",
			key:     "1234567890123",
			wantErr: true,
			errMsg:  "14 dígitos",
		},
		{
			name:    "CNPJ inválido — mais de 14 dígitos",
			pixType: "CNPJ",
			key:     "123456789012345",
			wantErr: true,
			errMsg:  "14 dígitos",
		},
		{
			name:    "CNPJ inválido — contém letra",
			pixType: "CNPJ",
			key:     "1234567800019a",
			wantErr: true,
			errMsg:  "apenas dígitos",
		},
		// ── PHONE ────────────────────────────────────────────────────────
		{
			name:    "PHONE válido com +55",
			pixType: "PHONE",
			key:     "+5511987654321",
			wantErr: false,
		},
		{
			name:    "PHONE inválido — sem +55",
			pixType: "PHONE",
			key:     "11987654321",
			wantErr: true,
			errMsg:  "+55",
		},
		{
			name:    "PHONE inválido — prefixo errado",
			pixType: "PHONE",
			key:     "+5411987654321",
			wantErr: true,
			errMsg:  "+55",
		},
		// ── EMAIL ────────────────────────────────────────────────────────
		{
			name:    "EMAIL válido",
			pixType: "EMAIL",
			key:     "usuario@example.com",
			wantErr: false,
		},
		{
			name:    "EMAIL inválido — sem @",
			pixType: "EMAIL",
			key:     "usuariosemaroba",
			wantErr: true,
			errMsg:  "@",
		},
		// ── RANDOM ───────────────────────────────────────────────────────
		{
			name:    "RANDOM válido — 36 caracteres UUID",
			pixType: "RANDOM",
			key:     "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "RANDOM inválido — menos de 36 caracteres",
			pixType: "RANDOM",
			key:     "550e8400-e29b-41d4-a716",
			wantErr: true,
			errMsg:  "36 caracteres",
		},
		// ── Tipo inválido ─────────────────────────────────────────────────
		{
			name:    "tipo inválido retorna erro",
			pixType: "DESCONHECIDO",
			key:     "qualquer",
			wantErr: true,
			errMsg:  "inválido",
		},
		{
			name:    "tipo vazio retorna erro",
			pixType: "",
			key:     "qualquer",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePixKey(tt.pixType, tt.key)
			if tt.wantErr {
				require.Error(t, err, "ValidatePixKey(%q, %q) deve retornar erro", tt.pixType, tt.key)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg,
						"mensagem de erro deve conter %q", tt.errMsg)
				}
			} else {
				require.NoError(t, err, "ValidatePixKey(%q, %q) não deve retornar erro", tt.pixType, tt.key)
			}
		})
	}
}
