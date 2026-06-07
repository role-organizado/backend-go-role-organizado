// Package worker provides the Temporal worker registry and schedule initializer
// for the backend-go-role-organizado service.
package worker

import (
	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

// Registry holds all Temporal workers created for this process.
// It mirrors the Java TemporalWorkerRegistry: a central place to create,
// start, and stop workers so main.go only deals with one object.
type Registry struct {
	client  client.Client
	workers []sdkworker.Worker
}

// NewRegistry creates a Registry backed by the given Temporal client.
func NewRegistry(c client.Client) *Registry {
	return &Registry{client: c}
}

// NewWorker creates a new worker for taskQueue, registers it internally,
// and returns it so callers can register workflows and activities on it.
// Call Start() after all workflows/activities have been registered.
func (r *Registry) NewWorker(taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	w := sdkworker.New(r.client, taskQueue, opts)
	r.workers = append(r.workers, w)
	return w
}

// Start calls Start() on every registered worker.
// It must be called after all workflows and activities have been registered
// and before the HTTP server begins accepting traffic.
func (r *Registry) Start() error {
	for _, w := range r.workers {
		if err := w.Start(); err != nil {
			return err
		}
	}
	return nil
}

// Stop gracefully stops all registered workers.
// It is safe to call even when no workers have been registered.
func (r *Registry) Stop() {
	for _, w := range r.workers {
		w.Stop()
	}
}
