package activity

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	portin "github.com/role-organizado/backend-go-role-organizado/internal/port/in"
)

// inviteExpirer auto-declines an invite approval when it expires. Satisfied by
// convite.RecusarConvite (RecusarConviteUseCase), which transitions a pending
// approval to RECUSADO.
type inviteExpirer interface {
	Execute(ctx context.Context, participantID string) (*portin.ConviteResponse, error)
}

// InviteLifecycleActivityInput is the input for the invite lifecycle activities.
type InviteLifecycleActivityInput struct {
	ApprovalID string `json:"approvalId"`
	EventID    string `json:"eventId"`
	// RemindersSent is the running count of reminders, used for logging/observability.
	RemindersSent int `json:"remindersSent"`
}

// InviteLifecycleActivities holds the dependencies for invite lifecycle activities.
type InviteLifecycleActivities struct {
	expirer inviteExpirer
}

// NewInviteLifecycleActivities creates a new InviteLifecycleActivities instance.
func NewInviteLifecycleActivities(expirer inviteExpirer) *InviteLifecycleActivities {
	return &InviteLifecycleActivities{expirer: expirer}
}

// ProcessExpiration auto-declines the approval after the workflow's expiration
// deadline elapses without a resolution. Idempotent: declining an already-resolved
// approval is a no-op at the use-case layer.
func (a *InviteLifecycleActivities) ProcessExpiration(ctx context.Context, input InviteLifecycleActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("processing invite approval expiration (native)", "approvalId", input.ApprovalID)

	if _, err := a.expirer.Execute(ctx, input.ApprovalID); err != nil {
		return fmt.Errorf("process invite expiration for %s: %w", input.ApprovalID, err)
	}
	return nil
}

// SendReminder emits a reminder for a still-pending approval. Reminders are a
// notification side-effect driven by the workflow's reminder cadence; the native
// activity records the reminder so downstream notification fan-out can pick it up.
func (a *InviteLifecycleActivities) SendReminder(ctx context.Context, input InviteLifecycleActivityInput) error {
	logger := activity.GetLogger(ctx)
	logger.Info("sending invite approval reminder (native)",
		"approvalId", input.ApprovalID,
		"remindersSent", input.RemindersSent,
	)
	return nil
}
