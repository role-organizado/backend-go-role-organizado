// Package workflow contains Temporal workflow definitions and ID generators.
package workflow

import "fmt"

// OverdueInstallmentPrimaryID returns the workflow ID for a manual run scoped to a reference date.
// Format: overdue-installment-real-{referenceDate}
func OverdueInstallmentPrimaryID(referenceDate string) string {
	return fmt.Sprintf("overdue-installment-real-%s", referenceDate)
}
