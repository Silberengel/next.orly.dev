package acl

import (
	"interfaces.orly/acl"
	"utils.orly/atomic"
)

var Registry = &S{}

type S struct {
	ACL    []acl.I
	Active atomic.String
}

type A struct{ S }

func (s *S) Register(i acl.I) {
	(*s).ACL = append((*s).ACL, i)
}

func (s *S) Configure(cfg ...any) (err error) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			err = i.Configure(cfg...)
			return
		}
	}
	return err
}

func (s *S) GetAccessLevel(pub []byte) (level string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			level = i.GetAccessLevel(pub)
			break
		}
	}
	return
}

func (s *S) GetACLInfo() (name, description, documentation string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			name, description, documentation = i.GetACLInfo()
			break
		}
	}
	return
}

func (s *S) Type() (typ string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			typ = i.Type()
			break
		}
	}
	return
}
