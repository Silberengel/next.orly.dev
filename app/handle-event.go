package app

import (
	"fmt"
	"strings"

	acl "acl.orly"
	"encoders.orly/envelopes/authenvelope"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/envelopes/okenvelope"
	"encoders.orly/kind"
	"encoders.orly/reason"
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
	// // send a challenge to the client to auth if an ACL is active and not authed
	// if acl.Registry.Active.Load() != "none" && l.authedPubkey.Load() == nil {
	// 	log.D.F("sending challenge to %s", l.remote)
	// 	if err = authenvelope.NewChallengeWith(l.challenge.Load()).
	// 		Write(l); chk.E(err) {
	// 		// return
	// 	}
	// 	// ACL is enabled so return and wait for auth
	// 	// return
	// }
	// check permissions of user
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load())
	switch accessLevel {
	case "none":
		log.D.F("handle event: sending CLOSED to %s", l.remote)
		if err = okenvelope.NewFrom(
			env.Id(), false,
			reason.AuthRequired.F("auth required for write access"),
		).Write(l); chk.E(err) {
			// return
		}
		log.D.F("handle event: sending challenge to %s", l.remote)
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			return
		}
		return
	case "read":
		log.D.F("handle event: sending CLOSED to %s", l.remote)
		if err = okenvelope.NewFrom(
			env.Id(), false,
			reason.AuthRequired.F("auth required for write access"),
		).Write(l); chk.E(err) {
			// return
		}
		log.D.F("handle event: sending challenge to %s", l.remote)
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			// return
		}
		return
	default:
		// user has write access or better, continue
		log.D.F("user has %s access", accessLevel)
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
