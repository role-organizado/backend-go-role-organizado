package finance

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// FinanceSummary holds the computed financial summary for an event.
type FinanceSummary struct {
	ID                     string
	EventID                string
	Goal                   int64   // centavos
	Collected              int64   // centavos
	ProgressPercentage     float64 // 4 casas, HALF_UP
	AvailableForWithdrawal int64
	PendingWithdrawals     int64
	LastCalculatedAt       time.Time
}

// Participant represents a user's participation in an event.
type Participant struct {
	ID      string
	EventID string
	UserID  string
	Name    string
	Email   string
	// Status: ORGANIZACAO | AGUARDANDO_ACEITE | ATIVO | INATIVO
	Status string
}

// PaymentInstallment represents a single payment installment for an event participant.
type PaymentInstallment struct {
	ID            string
	EventID       string
	ParticipantID string
	Amount        int64  // centavos
	Status        string // PAID | PENDING | OVERDUE
	PaymentMethod string // PIX | BOLETO | CREDIT_CARD
	PaidAt        *time.Time
}

// LedgerEntry represents a single financial transaction in the event ledger.
type LedgerEntry struct {
	ID              string
	EventID         string
	Type            string
	Amount          int64
	Description     string
	OccurredAt      time.Time
	AccountingClass string
}

// PaymentAccount holds a user's PIX or bank account for receiving payments.
type PaymentAccount struct {
	ID         string
	UserID     string
	Type       string // PIX | BANK
	PixKey     string
	PixType    string // CPF|CNPJ|PHONE|EMAIL|RANDOM
	BankCode   string
	AgencyNum  string
	AccountNum string
	IsDefault  bool
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// FinanceOverview summarises the financial state of an event for listing purposes.
type FinanceOverview struct {
	EventID                string
	EventName              string
	Goal                   int64
	Collected              int64
	ProgressPercentage     float64
	AvailableForWithdrawal int64
	PendingWithdrawals     int64
}

// LedgerStatementPage is a paginated list of ledger entries.
type LedgerStatementPage struct {
	Entries       []LedgerEntry
	TotalElements int64
	TotalPages    int
	Page          int
	Size          int
}

// ParticipantPaymentStatus holds the payment status for a single participant.
type ParticipantPaymentStatus struct {
	ParticipantID string
	Name          string
	Email         string
	Status        string // PAID | PENDING | OVERDUE
	PaidAmount    int64
	PendingAmount int64
}

// HoldBalance describes the blocked/available balance breakdown for an event.
type HoldBalance struct {
	BlockedBalance       int64
	AvailableForOutbound int64
	NextReleaseDate      *time.Time
	BreakdownByMethod    map[string]int64
}

// EventPaymentStatus summarises paid/pending/overdue tallies for an event.
type EventPaymentStatus struct {
	EventID      string
	TotalPaid    int64
	TotalPending int64
	TotalOverdue int64
	PaidCount    int
	PendingCount int
}

// AuditEntry represents a single audit event in the event's audit trail.
type AuditEntry struct {
	ID          string
	EventID     string
	Action      string
	ActorID     string
	Description string
	OccurredAt  time.Time
}

// ---- Domain functions ----

// CalculateProgress returns collected/goal*100 rounded HALF_UP to 4 decimal places.
// Returns 0 if goal is 0 to avoid division by zero.
func CalculateProgress(collected, goal int64) float64 {
	if goal == 0 {
		return 0
	}
	raw := float64(collected) / float64(goal) * 100
	// math.Round rounds half away from zero — equivalent to HALF_UP for positive values.
	return math.Round(raw*10000) / 10000
}

// ValidatePixKey validates a PIX key according to its declared pixType.
// Supported types (case-insensitive): CPF, CNPJ, PHONE, EMAIL, RANDOM.
func ValidatePixKey(pixType, key string) error {
	switch strings.ToUpper(pixType) {
	case "CPF":
		if len(key) != 11 {
			return fmt.Errorf("chave PIX CPF deve ter 11 dígitos, recebido %d", len(key))
		}
		for _, c := range key {
			if c < '0' || c > '9' {
				return fmt.Errorf("chave PIX CPF deve conter apenas dígitos")
			}
		}
	case "CNPJ":
		if len(key) != 14 {
			return fmt.Errorf("chave PIX CNPJ deve ter 14 dígitos, recebido %d", len(key))
		}
		for _, c := range key {
			if c < '0' || c > '9' {
				return fmt.Errorf("chave PIX CNPJ deve conter apenas dígitos")
			}
		}
	case "PHONE":
		if !strings.HasPrefix(key, "+55") {
			return fmt.Errorf("chave PIX telefone deve começar com +55")
		}
	case "EMAIL":
		if !strings.Contains(key, "@") {
			return fmt.Errorf("chave PIX e-mail deve conter @")
		}
	case "RANDOM":
		if len(key) != 36 {
			return fmt.Errorf("chave PIX aleatória deve ter 36 caracteres (formato UUID), recebido %d", len(key))
		}
	default:
		return fmt.Errorf("tipo de chave PIX inválido: %s", pixType)
	}
	return nil
}
