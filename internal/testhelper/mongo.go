// Package testhelper provides shared Testcontainers setup for integration tests.
package testhelper

import (
	"context"
	"testing"

	"github.com/role-organizado/backend-go-role-organizado/internal/adapter/mongodb"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
)

// StartMongo spins up a MongoDB container and returns a connected mongodb.Client.
// The container is terminated when the test finishes.
// Use this in integration tests:
//
//	client := testhelper.StartMongo(t)
func StartMongo(t *testing.T) *mongodb.Client {
	t.Helper()

	ctx := context.Background()

	container, err := tcmongo.Run(ctx, "mongo:7.0")
	if err != nil {
		t.Fatalf("failed to start MongoDB container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("warning: failed to terminate MongoDB container: %v", err)
		}
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get MongoDB connection string: %v", err)
	}

	client, err := mongodb.Connect(ctx, uri, "testdb")
	if err != nil {
		t.Fatalf("failed to connect to MongoDB: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("warning: failed to disconnect MongoDB: %v", err)
		}
	})

	return client
}
