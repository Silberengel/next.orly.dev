package app

import (
	"errors"

	"encoders.orly/envelopes/closeenvelope"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// HandleClose processes a CLOSE envelope by unmarshalling the request,
// validates the presence of an <id> field, and signals cancellation for
// the associated listener through the server's publisher mechanism.
func (l *Listener) HandleClose(
	req []byte,
) (err error) {
	var rem []byte
	env := closeenvelope.New()
	if rem, err = env.Unmarshal(req); chk.E(err) {
		return
	}
	if len(rem) > 0 {
		log.I.F("extra '%s'", rem)
	}
	if len(env.ID) == 0 {
		return errors.New("CLOSE has no <id>")
	}
	l.publishers.Receive(
		&W{
			Cancel: true,
			remote: l.remote,
			Conn:   l.conn,
			Id:     string(env.ID),
		},
	)
	return
}
