// Package worker manages Temporal worker lifecycle for all task queues.
package worker

import (
	"fmt"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// Task queue names — must match the Java TemporalWorkerRegistry constants during coexistence.
const (
	OverdueInstallmentQueue = "OVERDUE_INSTALLMENT_QUEUE"
)

// Registry manages Temporal worker lifecycle backed by a single client.
type Registry struct {
	client client.Client
}

// NewRegistry constructs a Registry backed by the given Temporal client.
func NewRegistry(c client.Client) *Registry {
	return &Registry{client: c}
}

// StartOverdueInstallmentWorker creates and starts the worker for OVERDUE_INSTALLMENT_QUEUE.
// The caller owns stopping the returned Worker (call w.Stop() during graceful shutdown).
func (r *Registry) StartOverdueInstallmentWorker(acts *activity.OverdueInstallmentActivities) (sdkworker.Worker, error) {
	w := sdkworker.New(r.client, OverdueInstallmentQueue, sdkworker.Options{})

	w.RegisterWorkflow(workflow.OverdueInstallmentWorkflow)
	w.RegisterActivity(acts)

	if err := w.Start(); err != nil {
		return nil, fmt.Errorf("starting overdue installment worker: %w", err)
	}
	return w, nil
}
