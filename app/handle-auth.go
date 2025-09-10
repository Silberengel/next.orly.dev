package app

import (
	"encoders.orly/envelopes/authenvelope"
	"encoders.orly/envelopes/okenvelope"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"protocol.orly/auth"
)

func (l *Listener) HandleAuth(b []byte) (err error) {
	var rem []byte
	env := authenvelope.NewResponse()
	if rem, err = env.Unmarshal(b); chk.E(err) {
		return
	}
	defer func() {
		if env != nil && env.Event != nil {
			env.Event.Free()
		}
	}()
	if len(rem) > 0 {
		log.I.F("extra '%s'", rem)
	}
	var valid bool
	if valid, err = auth.Validate(
		env.Event, l.challenge.Load(),
		l.ServiceURL(l.req),
	); err != nil {
		e := err.Error()
		if err = Ok.Error(l, env, e); chk.E(err) {
			return
		}
		return
	} else if !valid {
		if err = Ok.Error(
			l, env, "auth response event is invalid",
		); chk.E(err) {
			return
		}
		return
	} else {
		if err = okenvelope.NewFrom(
			env.Event.ID, true,
		).Write(l); chk.E(err) {
			return
		}
		log.D.F(
			"%s authed to pubkey %0x", l.remote,
			env.Event.Pubkey,
		)
		l.authedPubkey.Store(env.Event.Pubkey)
	}
	return
}
