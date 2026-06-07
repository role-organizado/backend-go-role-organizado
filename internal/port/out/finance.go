package out

import (
	"context"
	"time"

	domain "github.com/role-organizado/backend-go-role-organizado/internal/domain/finance"
)

// AuditTrailRepository defines persistence for audit trail entries.
// Returns an empty list gracefully if the collection does not exist.
type AuditTrailRepository interface {
	FindByEventID(ctx context.Context, eventID string, page, size int) ([]domain.AuditEntry, int64, error)
}

// FinanceSummaryRepository defines persistence operations for event finance summaries.
type FinanceSummaryRepository interface {
	FindByEventID(ctx context.Context, eventID string) (*domain.FinanceSummary, error)
	Save(ctx context.Context, s *domain.FinanceSummary) (*domain.FinanceSummary, error)
	Update(ctx context.Context, s *domain.FinanceSummary) (*domain.FinanceSummary, error)
}

// LedgerEntryRepository defines persistence operations for event ledger entries.
// entryType is optional (nil means "all types"); from/to are optional date range filters.
type LedgerEntryRepository interface {
	FindByEventID(ctx context.Context, eventID string, entryType *string, from, to *time.Time, page, size int) ([]domain.LedgerEntry, int64, error)
}

// PaymentAccountRepository defines persistence operations for user payment accounts (PIX/bank).
type FinanceAccountRepository interface {
	FindByUserID(ctx context.Context, userID string) ([]domain.PaymentAccount, error)
	FindByID(ctx context.Context, id, userID string) (*domain.PaymentAccount, error)
	Save(ctx context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error)
	Update(ctx context.Context, a *domain.PaymentAccount) (*domain.PaymentAccount, error)
	// ClearDefault removes the is_default flag from all accounts belonging to a user.
	ClearDefault(ctx context.Context, userID string) error
	// SoftDelete marks a payment account as inactive (active=false).
	SoftDelete(ctx context.Context, id, userID string) error
}
