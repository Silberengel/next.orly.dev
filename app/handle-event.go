package app

import (
	"fmt"
	"strings"

	acl "acl.orly"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/kind"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	utils "utils.orly"
)

func (l *Listener) HandleEvent(msg []byte) (err error) {
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
	// if the event is a delete, process the delete
	if env.E.Kind == kind.EventDeletion.K {
		l.HandleDelete(env)
	} else {
		// check if the event was deleted
		if err = l.CheckForDeleted(env.E, l.Admins); err != nil {
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):len(err.Error())]
				if err = Ok.Error(
					l, env, errStr,
				); chk.E(err) {
					return
				}
			}
		}
	}
	// store the event
	log.I.F("saving event %0x, %s", env.E.ID, env.E.Serialize())
	if _, _, err = l.SaveEvent(l.Ctx, env.E); chk.E(err) {
		return
	}
	// if a follow list was saved, reconfigure ACLs now that it is persisted
	if env.E.Kind == kind.FollowList.K {
		if err = acl.Registry.Configure(); chk.E(err) {
		}
	}
	l.publishers.Deliver(env.E)
	// Send a success response storing
	if err = Ok.Ok(l, env, ""); chk.E(err) {
		return
	}
	log.D.F("saved event %0x", env.E.ID)
	return
}
