package acl

import (
	"lol.mleku.dev/log"
)

type None struct{}

func (n None) Configure(cfg ...any) (err error) { return }

func (n None) GetAccessLevel(pub []byte) (level string) {
	return "write"
}

func (n None) GetACLInfo() (name, description, documentation string) {
	return "none", "no ACL", "blanket write access for all clients"
}

func (n None) Type() string {
	return "none"
}

func (n None) Syncer() {}

func init() {
	log.T.F("registering none ACL")
	Registry.Register(new(None))
}
