// Package mongodb provides a MongoDB client connection pool for the backend.
// It uses the official Go driver v2 with graceful shutdown support.
package mongodb

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Client wraps a MongoDB client with the target database name.
type Client struct {
	client   *mongo.Client
	database string
}

// Connect creates a new MongoDB connection and pings the server to verify connectivity.
// The returned Client must be disconnected via Disconnect when no longer needed.
func Connect(ctx context.Context, uri, database string) (*Client, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(5 * time.Second).
		SetMaxPoolSize(100).
		SetMinPoolSize(5)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("connecting to mongodb: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("pinging mongodb: %w", err)
	}

	slog.Info("mongodb connected", "database", database)

	return &Client{
		client:   client,
		database: database,
	}, nil
}

// DB returns the target database.
func (c *Client) DB() *mongo.Database {
	return c.client.Database(c.database)
}

// Collection returns a collection from the target database.
func (c *Client) Collection(name string) *mongo.Collection {
	return c.client.Database(c.database).Collection(name)
}

// Ping checks that the MongoDB connection is alive.
func (c *Client) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.Ping(pingCtx, readpref.Primary())
}

// Disconnect gracefully closes the connection pool.
func (c *Client) Disconnect(ctx context.Context) error {
	slog.Info("mongodb disconnecting")
	return c.client.Disconnect(ctx)
}
