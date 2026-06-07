package worker

import (
	"fmt"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// Registry manages Temporal workers for this service.
// Each worker polls a task queue and executes registered workflows and activities.
type Registry struct {
	client  client.Client
	workers []sdkworker.Worker
}

// NewRegistry constructs a Registry backed by the given Temporal client.
func NewRegistry(c client.Client) *Registry {
	return &Registry{client: c}
}

// newWorker creates a new worker for the given task queue and appends it to the registry.
func (r *Registry) newWorker(taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	w := sdkworker.New(r.client, taskQueue, opts)
	r.workers = append(r.workers, w)
	return w
}

// RegisterSandboxWorker registers the SandboxWorkflow and SandboxActivity on SANDBOX_QUEUE.
// This is a POC worker used to validate the Temporal Go foundation E2E.
func (r *Registry) RegisterSandboxWorker(act *activity.SandboxActivity) {
	w := r.newWorker("SANDBOX_QUEUE", sdkworker.Options{})
	w.RegisterWorkflow(workflow.SandboxWorkflow)
	w.RegisterActivity(act)
}

// Start starts all registered workers. Workers begin polling their task queues.
// Returns the first error encountered, if any.
func (r *Registry) Start() error {
	for _, w := range r.workers {
		if err := w.Start(); err != nil {
			return fmt.Errorf("starting temporal worker: %w", err)
		}
	}
	return nil
}

// Stop gracefully stops all registered workers.
func (r *Registry) Stop() {
	for _, w := range r.workers {
		w.Stop()
	}
}
