package app

import (
	"fmt"

	"encoders.orly/envelopes"
	"encoders.orly/envelopes/authenvelope"
	"encoders.orly/envelopes/closeenvelope"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/envelopes/noticeenvelope"
	"encoders.orly/envelopes/reqenvelope"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
)

func (l *Listener) HandleMessage(msg []byte, remote string) {
	log.D.C(
		func() string {
			return fmt.Sprintf(
				"%s received message:\n%s", remote, msg,
			)
		},
	)
	var err error
	var t string
	var rem []byte
	if t, rem, err = envelopes.Identify(msg); !chk.E(err) {
		switch t {
		case eventenvelope.L:
			log.D.F("eventenvelope: %s", rem)
			err = l.HandleEvent(l.ctx, rem)
		case reqenvelope.L:
			log.D.F("reqenvelope: %s", rem)
			err = l.HandleReq(l.ctx, rem)
		case closeenvelope.L:
			log.D.F("closeenvelope: %s", rem)
			err = l.HandleClose(rem)
		case authenvelope.L:
			log.D.F("authenvelope: %s", rem)
		default:
			err = errorf.E("unknown envelope type %s\n%s", t, rem)
		}
	}
	if err != nil {
		log.D.C(
			func() string {
				return fmt.Sprintf(
					"notice->%s %s", remote, err,
				)
			},
		)
		if err = noticeenvelope.NewFrom(err.Error()).Write(l); chk.E(err) {
			return
		}
	}

}
