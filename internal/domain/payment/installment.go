package payment

import "time"

// InstallmentStatus represents the lifecycle state of a payment installment.
type InstallmentStatus string

const (
	InstallmentStatusPending    InstallmentStatus = "PENDING"
	InstallmentStatusProcessing InstallmentStatus = "PROCESSING"
	InstallmentStatusPaid       InstallmentStatus = "PAID"
	InstallmentStatusOverdue    InstallmentStatus = "OVERDUE"
	InstallmentStatusCancelled  InstallmentStatus = "CANCELLED"
)

// PaymentInstallment represents a single installment in a participant's payment plan.
// AmountCents is stored as int64 to avoid floating-point errors.
type PaymentInstallment struct {
	ID                      string
	EventID                 string
	LiabilityID             string
	ParticipantID           string
	InstallmentNumber       int
	TotalInstallments       int
	AmountCents             int64
	DueDate                 time.Time
	Status                  InstallmentStatus
	TransactionID           string
	PaidAt                  *time.Time
	PaymentMethod           string
	PaymentReference        string
	OverdueNotificationSent bool
	CreatedAt               time.Time
	UpdatedAt               time.Time
}
