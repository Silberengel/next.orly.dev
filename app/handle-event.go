package app

import (
	"context"
	"fmt"

	"encoders.orly/envelopes/eventenvelope"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	utils "utils.orly"
)

func (l *Listener) HandleEvent(c context.Context, msg []byte) (
	err error,
) {
	// decode the envelope
	env := eventenvelope.NewSubmission()
	if msg, err = env.Unmarshal(msg); chk.E(err) {
		return
	}
	if len(msg) > 0 {
		log.I.F("extra '%s'", msg)
	}
	// check the event ID is correct
	calculatedId := env.E.GetIDBytes()
	if !utils.FastEqual(calculatedId, env.E.ID) {
		if err = Ok.Invalid(
			l, env, "event id is computed incorrectly, "+
				"event has ID %0x, but when computed it is %0x",
			env.E.ID, calculatedId,
		); chk.E(err) {
			return
		}
		return
	}
	// verify the signature
	var ok bool
	if ok, err = env.Verify(); chk.T(err) {
		if err = Ok.Error(
			l, env, fmt.Sprintf(
				"failed to verify signature: %s",
				err.Error(),
			),
		); chk.E(err) {
			return
		}
	} else if !ok {
		if err = Ok.Invalid(
			l, env,
			"signature is invalid",
		); chk.E(err) {
			return
		}
		return
	}
	// store the event
	if _, _, err = l.SaveEvent(c, env.E, false, nil); chk.E(err) {
		return
	}
	l.publishers.Deliver(env.E)
	// Send a success response storing
	if err = Ok.Ok(l, env, ""); chk.E(err) {
		return
	}
	log.D.F("saved event %0x", env.E.ID)
	return
}
