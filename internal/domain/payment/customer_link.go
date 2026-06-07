package payment

import "time"

// AsaasCustomerLink maps a platform user to their corresponding Asaas customer ID.
// The link is created lazily on first payment and reused for all subsequent charges.
type AsaasCustomerLink struct {
	UserID          string
	AsaasCustomerID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
