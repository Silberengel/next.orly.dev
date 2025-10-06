package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/countenvelope"
	"next.orly.dev/pkg/utils/normalize"
)

// HandleCount processes a COUNT envelope by parsing the request, verifying
// permissions, invoking the database CountEvents for each provided filter, and
// responding with a COUNT response containing the aggregate count.
func (l *Listener) HandleCount(msg []byte) (err error) {
	log.D.F("HandleCount: START processing from %s", l.remote)

	// Parse the COUNT request
	env := countenvelope.New()
	if _, err = env.Unmarshal(msg); chk.E(err) {
		return normalize.Error.Errorf(err.Error())
	}
 log.D.C(func() string { return fmt.Sprintf("COUNT sub=%s filters=%d", env.Subscription, len(env.Filters)) })

	// If ACL is active, send a challenge (same as REQ path)
	if acl.Registry.Active.Load() != "none" {
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).Write(l); chk.E(err) {
			return
		}
	}

	// Check read permissions
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)
	switch accessLevel {
	case "none":
		return errors.New("auth required: user not authed or has no read access")
	default:
		// allowed to read
	}

	// Use a bounded context for counting
	ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
	defer cancel()

	// Aggregate count across all provided filters
	var total int
	var approx bool // database returns false per implementation
 for _, f := range env.Filters {
		if f == nil {
			continue
		}
		var cnt int
		var a bool
		cnt, a, err = l.D.CountEvents(ctx, f)
		if chk.E(err) {
			return
		}
		total += cnt
		approx = approx || a
	}

	// Build and send COUNT response
	var res *countenvelope.Response
	if res, err = countenvelope.NewResponseFrom(env.Subscription, total, approx); chk.E(err) {
		return
	}
	if err = res.Write(l); chk.E(err) {
		return
	}

	log.D.F("HandleCount: COMPLETED processing from %s count=%d approx=%v", l.remote, total, approx)
	return nil
}
