// Package handler provides the health check HTTP handler.
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// HealthChecker is implemented by any component that can report its health.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthResponse matches the Spring Boot Actuator health response format
// so the BFF doesn't need to change routing logic.
type HealthResponse struct {
	Status     string                    `json:"status"`
	Components map[string]ComponentCheck `json:"components,omitempty"`
}

// ComponentCheck holds the status of a single infrastructure component.
type ComponentCheck struct {
	Status  string `json:"status"`
	Details any    `json:"details,omitempty"`
}

// HealthHandler returns an http.HandlerFunc that performs a liveness/readiness check.
// It responds with {"status":"UP"} (200) or {"status":"DOWN"} (503).
func HealthHandler(mongo HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := HealthResponse{
			Status:     "UP",
			Components: make(map[string]ComponentCheck),
		}
		overallUp := true

		if err := mongo.Ping(r.Context()); err != nil {
			slog.WarnContext(r.Context(), "mongodb health check failed", "error", err)
			resp.Components["mongo"] = ComponentCheck{Status: "DOWN", Details: err.Error()}
			overallUp = false
		} else {
			resp.Components["mongo"] = ComponentCheck{Status: "UP"}
		}

		if !overallUp {
			resp.Status = "DOWN"
		}

		statusCode := http.StatusOK
		if !overallUp {
			statusCode = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.ErrorContext(r.Context(), "encoding health response", "error", err)
		}
	}
}
