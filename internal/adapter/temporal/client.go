// Package temporal provides a Temporal client adapter for the backend.
// It creates a singleton client connected to the Temporal frontend service
// using the project's TemporalConfig settings.
package temporal

import (
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"

	"github.com/role-organizado/backend-go-role-organizado/internal/config"
)

// NewClient creates and returns a connected Temporal client.
// The caller is responsible for calling client.Close() when the application shuts down.
func NewClient(cfg config.TemporalConfig) (client.Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dialing temporal at %s (namespace %s): %w", cfg.HostPort, cfg.Namespace, err)
	}

	slog.Info("temporal client connected", "host_port", cfg.HostPort, "namespace", cfg.Namespace)

	return c, nil
}
