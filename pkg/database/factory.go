package database

import (
	"context"
	"fmt"
	"strings"
)

// NewDatabase creates a database instance based on the specified type.
// Supported types: "badger", "dgraph"
func NewDatabase(
	ctx context.Context,
	cancel context.CancelFunc,
	dbType string,
	dataDir string,
	logLevel string,
) (Database, error) {
	switch strings.ToLower(dbType) {
	case "badger", "":
		// Use the existing badger implementation
		return New(ctx, cancel, dataDir, logLevel)
	case "dgraph":
		// Use the new dgraph implementation
		// Import dynamically to avoid import cycles
		return newDgraphDatabase(ctx, cancel, dataDir, logLevel)
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: badger, dgraph)", dbType)
	}
}

// newDgraphDatabase creates a dgraph database instance
// This is defined here to avoid import cycles
var newDgraphDatabase func(context.Context, context.CancelFunc, string, string) (Database, error)

// RegisterDgraphFactory registers the dgraph database factory
// This is called from the dgraph package's init() function
func RegisterDgraphFactory(factory func(context.Context, context.CancelFunc, string, string) (Database, error)) {
	newDgraphDatabase = factory
}
