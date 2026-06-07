package in

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
)

// ---- Input types ----

// ListFinanceEventsInput holds the input for listing finance overviews for a user.
type ListFinanceEventsInput struct {
	UserID string
}

// GetFinanceOverviewInput holds the input for retrieving a single event's finance overview.
type GetFinanceOverviewInput struct {
	EventID string
	UserID  string
}

// GetLedgerStatementInput holds the input for retrieving a paginated ledger statement.
type GetLedgerStatementInput struct {
	EventID string
	UserID  string
	Type    string     // "all" | "INCOME" | "EXPENSE" | ...
	From    *time.Time
	To      *time.Time
	Page    int // >= 0
	Size    int // clamped to 1..100
}

// GetParticipantsStatusInput holds the input for listing participants' payment statuses.
type GetParticipantsStatusInput struct {
	EventID string
	UserID  string
	Page    int
	Size    int
}

// RecalculateFinanceSummaryInput holds the input for triggering a finance summary recalculation.
type RecalculateFinanceSummaryInput struct {
	EventID string
	UserID  string
}

// SendPaymentRemindersInput holds the input for sending payment reminders to pending participants.
type SendPaymentRemindersInput struct {
	EventID string
	UserID  string
}

// CalculateHoldBalanceInput holds the input for computing the hold/blocked balance.
type CalculateHoldBalanceInput struct {
	EventID string
	UserID  string
}

// GetEventPaymentStatusInput holds the input for retrieving the aggregated payment status.
type GetEventPaymentStatusInput struct {
	EventID string
	UserID  string
}

// GetAuditTrailInput holds the input for retrieving the audit trail for an event.
type GetAuditTrailInput struct {
	EventID string
	UserID  string
	Page    int
	Size    int
}

// ListPaymentAccountsInput holds the input for listing a user's payment accounts.
type ListPaymentAccountsInput struct {
	UserID string
}

// CreatePaymentAccountInput holds the data required to create a new payment account.
type CreatePaymentAccountInput struct {
	UserID     string
	Type       string // PIX | BANK
	PixKey     string
	PixType    string // CPF | CNPJ | PHONE | EMAIL | RANDOM
	BankCode   string
	AgencyNum  string
	AccountNum string
}

// UpdatePaymentAccountInput holds the data required to update an existing payment account.
type UpdatePaymentAccountInput struct {
	AccountID  string
	UserID     string
	Type       string
	PixKey     string
	PixType    string
	BankCode   string
	AgencyNum  string
	AccountNum string
}

// ---- Use case interfaces ----

// ListFinanceEventsUseCase lists finance overviews for all events belonging to a user.
type ListFinanceEventsUseCase interface {
	Execute(ctx context.Context, in ListFinanceEventsInput) ([]domain.FinanceOverview, error)
}

// GetFinanceOverviewUseCase returns the finance overview for a single event.
type GetFinanceOverviewUseCase interface {
	Execute(ctx context.Context, in GetFinanceOverviewInput) (*domain.FinanceOverview, error)
}

// GetLedgerStatementUseCase returns a paginated ledger statement for an event.
type GetLedgerStatementUseCase interface {
	Execute(ctx context.Context, in GetLedgerStatementInput) (*domain.LedgerStatementPage, error)
}

// GetParticipantsStatusUseCase returns payment statuses for event participants.
// Returns the status slice, total element count, and error.
type GetParticipantsStatusUseCase interface {
	Execute(ctx context.Context, in GetParticipantsStatusInput) ([]domain.ParticipantPaymentStatus, int64, error)
}

// RecalculateFinanceSummaryUseCase triggers a recalculation of the event's finance summary.
type RecalculateFinanceSummaryUseCase interface {
	Execute(ctx context.Context, in RecalculateFinanceSummaryInput) (*domain.FinanceSummary, error)
}

// SendPaymentRemindersUseCase enqueues payment reminder notifications for pending participants.
type SendPaymentRemindersUseCase interface {
	Execute(ctx context.Context, in SendPaymentRemindersInput) error
}

// CalculateHoldBalanceUseCase calculates the hold/blocked balance for an event.
type CalculateHoldBalanceUseCase interface {
	Execute(ctx context.Context, in CalculateHoldBalanceInput) (*domain.HoldBalance, error)
}

// GetEventPaymentStatusUseCase returns the aggregated payment status for an event.
type GetEventPaymentStatusUseCase interface {
	Execute(ctx context.Context, in GetEventPaymentStatusInput) (*domain.EventPaymentStatus, error)
}

// ManagePaymentAccountsUseCase handles CRUD operations for user PIX/bank payment accounts.
type ManagePaymentAccountsUseCase interface {
	List(ctx context.Context, in ListPaymentAccountsInput) ([]domain.PaymentAccount, error)
	Create(ctx context.Context, in CreatePaymentAccountInput) (*domain.PaymentAccount, error)
	Update(ctx context.Context, in UpdatePaymentAccountInput) (*domain.PaymentAccount, error)
	SetDefault(ctx context.Context, accountID, userID string) error
	Delete(ctx context.Context, accountID, userID string) error
}

// GetAuditTrailUseCase returns the paginated audit trail for an event.
type GetAuditTrailUseCase interface {
	Execute(ctx context.Context, in GetAuditTrailInput) ([]domain.AuditEntry, int64, error)
}
