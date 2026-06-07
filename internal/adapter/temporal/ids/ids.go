// Package ids provides canonical workflow ID generators for Temporal workflows.
package ids

import "fmt"

// FinanceReconciliationPrimaryID returns the canonical workflow ID for a manual
// finance reconciliation run.
// Format: finance-reconciliation-real-{referenceDate}
func FinanceReconciliationPrimaryID(referenceDate string) string {
	return fmt.Sprintf("finance-reconciliation-real-%s", referenceDate)
}
