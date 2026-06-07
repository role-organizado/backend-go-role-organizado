// Package worker provides Temporal worker registration and lifecycle management.
package worker

import (
	"fmt"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

// Registry manages a collection of Temporal workers, providing a unified Start/Stop
// lifecycle. Equivalent to the Java TemporalWorkerRegistry pattern.
type Registry struct {
	client  client.Client
	workers []sdkworker.Worker
}

// NewRegistry creates a new worker registry backed by the provided Temporal client.
func NewRegistry(c client.Client) *Registry {
	return &Registry{client: c}
}

// NewWorker creates a new Temporal worker for the given task queue and appends it
// to the registry. The worker is NOT started until Start() is called.
func (r *Registry) NewWorker(taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	w := sdkworker.New(r.client, taskQueue, opts)
	r.workers = append(r.workers, w)
	return w
}

// Start starts all registered workers. Workers begin polling their task queues.
func (r *Registry) Start() error {
	for i, w := range r.workers {
		if err := w.Start(); err != nil {
			for j := 0; j < i; j++ {
				r.workers[j].Stop()
			}
			return fmt.Errorf("start worker %d: %w", i, err)
		}
	}
	return nil
}

// Stop stops all registered workers gracefully.
func (r *Registry) Stop() {
	for i := len(r.workers) - 1; i >= 0; i-- {
		r.workers[i].Stop()
	}
}

// Client returns the underlying Temporal client, useful for schedule management.
func (r *Registry) Client() client.Client {
	return r.client
}
