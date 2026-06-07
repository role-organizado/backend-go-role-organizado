// Package worker — schedules.go
// ScheduleInitializer is responsible for creating or updating Temporal schedule
// objects (equivalent to Java's TemporalScheduleInitializer @PostConstruct bean).
// The implementation is intentionally empty at this stage; it will be populated
// as each scheduled workflow wave (T004 PspReconciliation, T005 Finance/Overdue,
// T006 PricingPspReview) is migrated.
package worker

import "go.temporal.io/sdk/client"

// ScheduleInitializer creates or updates Temporal schedules on startup.
// Add one Initialize* method per scheduled workflow as each wave is implemented.
type ScheduleInitializer struct {
	client client.Client
}

// NewScheduleInitializer creates a ScheduleInitializer backed by the given client.
func NewScheduleInitializer(c client.Client) *ScheduleInitializer {
	return &ScheduleInitializer{client: c}
}
