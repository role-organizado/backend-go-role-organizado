package activity

import (
	"context"
	"log/slog"
)

// SandboxActivity holds the POC activity used to validate the Temporal Go foundation E2E.
type SandboxActivity struct{}

// NewSandboxActivity constructs a SandboxActivity.
func NewSandboxActivity() *SandboxActivity {
	return &SandboxActivity{}
}

// EnviarLogAssincrono logs the received message asynchronously and returns a confirmation string.
func (a *SandboxActivity) EnviarLogAssincrono(ctx context.Context, message string) (string, error) {
	slog.Info("[sandbox] activity executed", "message", message)
	return "logged: " + message, nil
}
