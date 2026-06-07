package payment

import "time"

// AccountType identifies whether the account is a PIX key or a bank account.
type AccountType string

const (
	AccountTypePix         AccountType = "PIX"
	AccountTypeBankAccount AccountType = "BANK_ACCOUNT"
)

// PixKeyType identifies the kind of PIX key.
type PixKeyType string

const (
	PixKeyTypeCPF            PixKeyType = "CPF"
	PixKeyTypeCNPJ           PixKeyType = "CNPJ"
	PixKeyTypeEmail          PixKeyType = "EMAIL"
	PixKeyTypeTelefone       PixKeyType = "TELEFONE"
	PixKeyTypeChaveAleatoria PixKeyType = "CHAVE_ALEATORIA"
)

// PaymentAccount represents a PIX key or bank account linked to a user for
// receiving payments (organiser payout account).
type PaymentAccount struct {
	ID                    string
	UserID                string
	AccountType           AccountType
	PixKeyType            PixKeyType
	PixKey                string
	BankCode              string
	BankName              string
	Agency                string
	AccountNumber         string
	AccountDigit          string
	AccountHolderName     string
	AccountHolderDocument string
	IsDefault             bool
	IsActive              bool
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
