package app

import (
	"fmt"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/encoders/envelopes"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/closeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/noticeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/reqenvelope"
)

func (l *Listener) HandleMessage(msg []byte, remote string) {
	// log.D.F("%s received message:\n%s", remote, msg)
	var err error
	var t string
	var rem []byte
	if t, rem, err = envelopes.Identify(msg); !chk.E(err) {
		switch t {
		case eventenvelope.L:
			// log.D.F("eventenvelope: %s %s", remote, rem)
			err = l.HandleEvent(rem)
		case reqenvelope.L:
			// log.D.F("reqenvelope: %s %s", remote, rem)
			err = l.HandleReq(rem)
		case closeenvelope.L:
			// log.D.F("closeenvelope: %s %s", remote, rem)
			err = l.HandleClose(rem)
		case authenvelope.L:
			// log.D.F("authenvelope: %s %s", remote, rem)
			err = l.HandleAuth(rem)
		default:
			err = fmt.Errorf("unknown envelope type %s\n%s", t, rem)
		}
	}
	if err != nil {
		// log.D.C(
		// 	func() string {
		// 		return fmt.Sprintf(
		// 			"notice->%s %s", remote, err,
		// 		)
		// 	},
		// )
		if err = noticeenvelope.NewFrom(err.Error()).Write(l); err != nil {
			return
		}
	}

}
