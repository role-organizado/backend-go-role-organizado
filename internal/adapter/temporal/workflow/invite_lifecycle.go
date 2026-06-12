package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
)

// InviteLifecycleInput is the input for InviteLifecycleWorkflow.
type InviteLifecycleInput struct {
	ApprovalID string `json:"approvalId"`
	EventID    string `json:"eventId"`
	// ReminderIntervalSeconds is the cadence between reminders while awaiting a
	// resolution. When <= 0 a 12-hour default is used.
	ReminderIntervalSeconds int `json:"reminderIntervalSeconds"`
	// ExpirationSeconds is the deadline after which the approval is auto-declined.
	// When <= 0 a 72-hour default is used.
	ExpirationSeconds int `json:"expirationSeconds"`
}

// InviteResolution is the payload of the resolveApproval signal.
type InviteResolution struct {
	// Status is the resolved approval status (e.g. "APPROVED", "REJECTED").
	Status     string `json:"status"`
	ResolverID string `json:"resolverId"`
}

// InviteLifecycleState is the observable state of an invite lifecycle run.
type InviteLifecycleState struct {
	// Status is one of "PENDING", "RESOLVED", "CANCELLED", or "EXPIRED".
	Status string `json:"status"`
	// ApprovalStatus mirrors the domain approval status.
	ApprovalStatus string `json:"approvalStatus"`
	ApprovalID     string `json:"approvalId"`
	RemindersSent  int    `json:"remindersSent"`
}

const (
	defaultInviteReminderInterval = 12 * time.Hour
	defaultInviteExpiration       = 72 * time.Hour
)

// InviteLifecycleWorkflow models a pending invite-approval awaiting an organizer
// decision. It sends periodic reminders, auto-declines on expiration, and
// resolves/cancels on signal.
//
// Versioning: gated on change ID "invite-lifecycle-v1".
// Signals:    resolveApproval, cancelApproval
// Queries:    getWorkflowStatus, getCurrentApprovalStatus
func InviteLifecycleWorkflow(ctx workflow.Context, input InviteLifecycleInput) error {
	// Version gate — reserved for future behavioral changes; v1 is the baseline.
	_ = workflow.GetVersion(ctx, "invite-lifecycle-v1", workflow.DefaultVersion, 1)

	state := InviteLifecycleState{
		Status:         "PENDING",
		ApprovalStatus: "PENDING_APPROVAL",
		ApprovalID:     input.ApprovalID,
	}

	_ = workflow.SetQueryHandler(ctx, "getWorkflowStatus", func() (string, error) {
		return state.Status, nil
	})
	_ = workflow.SetQueryHandler(ctx, "getCurrentApprovalStatus", func() (string, error) {
		return state.ApprovalStatus, nil
	})

	reminderInterval := defaultInviteReminderInterval
	if input.ReminderIntervalSeconds > 0 {
		reminderInterval = time.Duration(input.ReminderIntervalSeconds) * time.Second
	}
	expiration := defaultInviteExpiration
	if input.ExpirationSeconds > 0 {
		expiration = time.Duration(input.ExpirationSeconds) * time.Second
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	actCtx := workflow.WithActivityOptions(ctx, ao)

	resolveCh := workflow.GetSignalChannel(ctx, "resolveApproval")
	cancelCh := workflow.GetSignalChannel(ctx, "cancelApproval")

	expCtx, cancelExp := workflow.WithCancel(ctx)
	defer cancelExp()
	expTimer := workflow.NewTimer(expCtx, expiration)

	done := false
	expired := false
	for !done {
		reminderCtx, cancelReminder := workflow.WithCancel(ctx)
		reminderFired := false

		sel := workflow.NewSelector(ctx)
		sel.AddReceive(resolveCh, func(c workflow.ReceiveChannel, _ bool) {
			var res InviteResolution
			c.Receive(ctx, &res)
			if res.Status != "" {
				state.ApprovalStatus = res.Status
			} else {
				state.ApprovalStatus = "APPROVED"
			}
			state.Status = "RESOLVED"
			done = true
		})
		sel.AddReceive(cancelCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, nil)
			state.Status = "CANCELLED"
			state.ApprovalStatus = "CANCELLED"
			done = true
		})
		sel.AddFuture(workflow.NewTimer(reminderCtx, reminderInterval), func(workflow.Future) {
			reminderFired = true
		})
		sel.AddFuture(expTimer, func(workflow.Future) {
			expired = true
			done = true
		})

		sel.Select(ctx)
		cancelReminder()

		if reminderFired && !done {
			actInput := activity.InviteLifecycleActivityInput{
				ApprovalID:    input.ApprovalID,
				EventID:       input.EventID,
				RemindersSent: state.RemindersSent,
			}
			if err := workflow.ExecuteActivity(actCtx, "SendReminder", actInput).Get(ctx, nil); err != nil {
				return fmt.Errorf("invite reminder: %w", err)
			}
			state.RemindersSent++
		}
	}
	cancelExp()

	if expired {
		actInput := activity.InviteLifecycleActivityInput{
			ApprovalID:    input.ApprovalID,
			EventID:       input.EventID,
			RemindersSent: state.RemindersSent,
		}
		if err := workflow.ExecuteActivity(actCtx, "ProcessExpiration", actInput).Get(ctx, nil); err != nil {
			return fmt.Errorf("invite expiration: %w", err)
		}
		state.Status = "EXPIRED"
		state.ApprovalStatus = "EXPIRED"
	}

	return nil
}
