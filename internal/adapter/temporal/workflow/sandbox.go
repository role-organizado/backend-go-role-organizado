package workflow

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// ── Activity interface ────────────────────────────────────────────────────────

// SandboxActivities holds the activity methods for the sandbox (POC) workflow.
// Real implementations inject dependencies; in tests all methods are mocked.
type SandboxActivities struct{}

// EnviarLogAssincrono asynchronously logs the event name.
// Mirrors Java's SandboxActivities.enviarLogAssincrono.
func (a *SandboxActivities) EnviarLogAssincrono(ctx context.Context, nome string) error {
	activity.GetLogger(ctx).Info("sandbox activity executed", "nome", nome)
	return nil
}

// ── Workflow ─────────────────────────────────────────────────────────────────

// SandboxWorkflow is a POC workflow that calls EnviarLogAssincrono.
// It is NOT a production workflow — used to validate the Temporal Go scaffold.
// Mirrors Java's SandboxWorkflowImpl.iniciarRoteiro (message parameter only).
func SandboxWorkflow(ctx workflow.Context, message string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("[SANDBOX] Starting workflow", "message", message)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var a *SandboxActivities
	err := workflow.ExecuteActivity(ctx, a.EnviarLogAssincrono, message).Get(ctx, nil)
	if err != nil {
		logger.Error("[SANDBOX] Activity failed", "error", err)
		return err
	}

	logger.Info("[SANDBOX] Workflow completed", "message", message)
	return nil
}
