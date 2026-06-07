// Package worker manages Temporal worker lifecycle for the Go backend.
package worker

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
)

// Registry manages a pool of Temporal workers sharing a single client connection.
type Registry struct {
	client  client.Client
	workers []sdkworker.Worker
}

// NewRegistry creates a Registry backed by the given Temporal client.
func NewRegistry(c client.Client) *Registry {
	return &Registry{client: c}
}

// NewWorker creates and registers a new worker for the given task queue.
// The worker is accumulated internally and started with Start().
func (r *Registry) NewWorker(taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	w := sdkworker.New(r.client, taskQueue, opts)
	r.workers = append(r.workers, w)
	return w
}

// RegisterPricingPspReviewWorker registers the PricingPspReview workflow and activity
// on the PRICING_PSP_REVIEW_QUEUE task queue.
func (r *Registry) RegisterPricingPspReviewWorker(act *activity.PricingPspReviewActivity) {
	w := r.NewWorker("PRICING_PSP_REVIEW_QUEUE", sdkworker.Options{})
	w.RegisterWorkflow(workflow.PricingPspReviewWorkflow)
	w.RegisterActivity(act)
}

// Start launches all registered workers in the background.
// Returns the first error encountered; subsequent workers are not started.
func (r *Registry) Start() error {
	for _, w := range r.workers {
		if err := w.Start(); err != nil {
			return fmt.Errorf("starting temporal worker: %w", err)
		}
	}
	slog.Info("temporal workers started", "count", len(r.workers))
	return nil
}

// Stop gracefully stops all registered workers and drains in-flight activity/workflow tasks.
func (r *Registry) Stop() {
	for _, w := range r.workers {
		w.Stop()
	}
	slog.Info("temporal workers stopped", "count", len(r.workers))
}
