package dgraph

import (
	"context"
	"fmt"

	"github.com/dgraph-io/dgo/v230/protos/api"
)

// NostrSchema defines the Dgraph schema for Nostr events
const NostrSchema = `
# Event node type
type Event {
	event.id
	event.serial
	event.kind
	event.created_at
	event.content
	event.sig
	event.pubkey
	event.authored_by
	event.references
	event.mentions
	event.tagged_with
}

# Author node type
type Author {
	author.pubkey
	author.events
}

# Tag node type
type Tag {
	tag.type
	tag.value
	tag.events
}

# Marker node type (for key-value metadata)
type Marker {
	marker.key
	marker.value
}

# Event fields
event.id: string @index(exact) @upsert .
event.serial: int @index(int) .
event.kind: int @index(int) .
event.created_at: int @index(int) .
event.content: string .
event.sig: string @index(exact) .
event.pubkey: string @index(exact) .

# Event relationships
event.authored_by: uid @reverse .
event.references: [uid] @reverse .
event.mentions: [uid] @reverse .
event.tagged_with: [uid] @reverse .

# Author fields
author.pubkey: string @index(exact) @upsert .
author.events: [uid] @count @reverse .

# Tag fields
tag.type: string @index(exact) .
tag.value: string @index(exact, fulltext) .
tag.events: [uid] @count @reverse .

# Marker fields (key-value storage)
marker.key: string @index(exact) @upsert .
marker.value: string .
`

// applySchema applies the Nostr schema to the connected Dgraph instance
func (d *D) applySchema(ctx context.Context) error {
	d.Logger.Infof("applying Nostr schema to dgraph")

	op := &api.Operation{
		Schema: NostrSchema,
	}

	if err := d.client.Alter(ctx, op); err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}

	d.Logger.Infof("schema applied successfully")
	return nil
}

// dropAll drops all data from dgraph (useful for testing)
func (d *D) dropAll(ctx context.Context) error {
	d.Logger.Warningf("dropping all data from dgraph")

	op := &api.Operation{
		DropAll: true,
	}

	if err := d.client.Alter(ctx, op); err != nil {
		return fmt.Errorf("failed to drop all data: %w", err)
	}

	// Reapply schema after dropping
	return d.applySchema(ctx)
}
