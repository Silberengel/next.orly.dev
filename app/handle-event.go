package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/okenvelope"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/reason"
	"next.orly.dev/pkg/utils"
)

func (l *Listener) HandleEvent(msg []byte) (err error) {
	// decode the envelope
	env := eventenvelope.NewSubmission()
	if msg, err = env.Unmarshal(msg); chk.E(err) {
		return
	}
	defer func() {
		if env != nil && env.E != nil {
			env.E.Free()
		}
	}()
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
	// check permissions of user
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)
	switch accessLevel {
	case "none":
		log.D.F(
			"handle event: sending 'OK,false,auth-required...' to %s", l.remote,
		)
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
		log.D.F(
			"handle event: sending 'OK,false,auth-required:...' to %s",
			l.remote,
		)
		if err = okenvelope.NewFrom(
			env.Id(), false,
			reason.AuthRequired.F("auth required for write access"),
		).Write(l); chk.E(err) {
			return
		}
		log.D.F("handle event: sending challenge to %s", l.remote)
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			return
		}
		return
	default:
		// user has write access or better, continue
		// log.D.F("user has %s access", accessLevel)
	}
	// if the event is a delete, process the delete
	if env.E.Kind == kind.EventDeletion.K {
		if err = l.HandleDelete(env); err != nil {
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):len(err.Error())]
				if err = Ok.Error(
					l, env, errStr,
				); chk.E(err) {
					return
				}
				return
			}
		}
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
	// store the event - use a separate context to prevent cancellation issues
	saveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// log.I.F("saving event %0x, %s", env.E.ID, env.E.Serialize())
	if _, _, err = l.SaveEvent(saveCtx, env.E); err != nil {
		if strings.HasPrefix(err.Error(), "blocked:") {
			errStr := err.Error()[len("blocked: "):len(err.Error())]
			if err = Ok.Error(
				l, env, errStr,
			); chk.E(err) {
				return
			}
			return
		}
		chk.E(err)
		return
	}
	// Send a success response storing
	if err = Ok.Ok(l, env, ""); chk.E(err) {
		return
	}
	// Deliver the event to subscribers immediately after sending OK response
	// Clone the event to prevent corruption when the original is freed
	clonedEvent := env.E.Clone()
	go l.publishers.Deliver(clonedEvent)
	log.D.F("saved event %0x", env.E.ID)
	var isNewFromAdmin bool
	for _, admin := range l.Admins {
		if utils.FastEqual(admin, env.E.Pubkey) {
			isNewFromAdmin = true
			break
		}
	}
	if isNewFromAdmin {
		log.I.F("new event from admin %0x", env.E.Pubkey)
		// if a follow list was saved, reconfigure ACLs now that it is persisted
		if env.E.Kind == kind.FollowList.K ||
			env.E.Kind == kind.RelayListMetadata.K {
			// Run ACL reconfiguration asynchronously to prevent blocking websocket operations
			go func() {
				if err := acl.Registry.Configure(); chk.E(err) {
					log.E.F("failed to reconfigure ACL: %v", err)
				}
			}()
		}
	}
	return
}
