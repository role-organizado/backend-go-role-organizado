// Package worker manages Temporal worker lifecycle and registration.
package worker

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"

	temporalactivity "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/activity"
	temporalworkflow "github.com/role-organizado/backend-go-role-organizado/internal/adapter/temporal/workflow"
	"github.com/role-organizado/backend-go-role-organizado/internal/config"
)

const (
	// FinanceReconciliationQueue is the Temporal task queue for finance reconciliation.
	FinanceReconciliationQueue = "FINANCE_RECONCILIATION_QUEUE"
)

// Registry manages the lifecycle of Temporal workers for this service.
type Registry struct {
	client         client.Client
	workers        []sdkworker.Worker
	javaBackendURL string
}

// New creates a Temporal client and returns a Registry ready for worker registration.
func New(cfg config.AppConfig) (*Registry, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal at %s: %w", cfg.Temporal.HostPort, err)
	}
	return &Registry{
		client:         c,
		javaBackendURL: cfg.Server.JavaBackendURL,
	}, nil
}

// Client returns the underlying Temporal client, e.g. for schedule management.
func (r *Registry) Client() client.Client {
	return r.client
}

// RegisterFinanceReconciliation registers the FINANCE_RECONCILIATION_QUEUE worker with
// the FinanceReconciliationWorkflow and RunReconciliation activity.
func (r *Registry) RegisterFinanceReconciliation() {
	activities := temporalactivity.NewFinanceReconciliationActivities(r.javaBackendURL)

	w := sdkworker.New(r.client, FinanceReconciliationQueue, sdkworker.Options{})
	w.RegisterWorkflow(temporalworkflow.FinanceReconciliationWorkflow)
	w.RegisterActivity(activities)

	r.workers = append(r.workers, w)
	slog.Info("registered temporal worker", "queue", FinanceReconciliationQueue)
}

// Start starts all registered workers. Workers begin polling their task queues.
func (r *Registry) Start() error {
	for _, w := range r.workers {
		if err := w.Start(); err != nil {
			return fmt.Errorf("starting temporal worker: %w", err)
		}
	}
	slog.Info("temporal workers started", "count", len(r.workers))
	return nil
}

// Stop stops all registered workers and closes the Temporal client.
// Safe to call even if Start was not called or returned an error.
func (r *Registry) Stop() {
	for _, w := range r.workers {
		w.Stop()
	}
	r.client.Close()
	slog.Info("temporal workers stopped")
}
