// Package acl is an interface for implementing arbitrary access control lists.
package acl

import (
	"interfaces.orly/typer"
)

const (
	// Read means read only
	Read = "read"
	// Write means read and write
	Write = "write"
	// Admin means read, write, import/export and arbitrary delete
	Admin = "admin"
	// Owner means read, write, import/export, arbitrary delete and wipe
	Owner = "owner"
	// Group applies to communities and other groups; the content afterwards a
	// set of comma separated <permission>:<pubkey> pairs designating permissions to groups.
	Group = "group:"
)

type I interface {
	Configure(cfg ...any) (err error)
	// GetAccessLevel returns the access level string for a given pubkey.
	GetAccessLevel(pub []byte, address string) (level string)
	// GetACLInfo returns the name and a description of the ACL, which should
	// explain briefly how it works, and then a long text of documentation of
	// the ACL's rules and configuration (in asciidoc or markdown).
	GetACLInfo() (name, description, documentation string)
	// Syncer is a worker thread that does things in the background like syncing
	// with other relays on admin relay lists using subscriptions for all events
	// that arrive elsewhere relevant to the ACL scheme.
	Syncer()
	typer.T
}
