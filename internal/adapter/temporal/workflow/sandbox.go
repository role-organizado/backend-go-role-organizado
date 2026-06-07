package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// SandboxWorkflow is a POC workflow that validates the Temporal Go foundation E2E.
// It executes a single activity (EnviarLogAssincrono) on SANDBOX_QUEUE and returns the result.
func SandboxWorkflow(ctx workflow.Context, message string) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	err := workflow.ExecuteActivity(ctx, "EnviarLogAssincrono", message).Get(ctx, &result)
	return result, err
}
