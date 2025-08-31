package app

import (
	"fmt"

	"encoders.orly/envelopes"
	"encoders.orly/envelopes/authenvelope"
	"encoders.orly/envelopes/closeenvelope"
	"encoders.orly/envelopes/eventenvelope"
	"encoders.orly/envelopes/reqenvelope"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

func (s *Server) HandleMessage(msg []byte, remote string) {
	log.D.C(
		func() string {
			return fmt.Sprintf(
				"%s received message:\n%s", remote, msg,
			)
		},
	)
	var notice []byte
	var err error
	var t string
	var rem []byte
	if t, rem, err = envelopes.Identify(msg); chk.E(err) {
		notice = []byte(err.Error())
	}
	switch t {
	case eventenvelope.L:
		log.D.F("eventenvelope: %s", rem)
	case reqenvelope.L:
		log.D.F("reqenvelope: %s", rem)
	case closeenvelope.L:
		log.D.F("closeenvelope: %s", rem)
	case authenvelope.L:
		log.D.F("authenvelope: %s", rem)
	default:
		notice = []byte(fmt.Sprintf("unknown envelope type %s\n%s", t, rem))
	}
	if len(notice) > 0 {
		log.D.C(
			func() string {
				return fmt.Sprintf(
					"notice->%s %s", remote, notice,
				)
			},
		)
		// if err = noticeenvelope.NewFrom(notice).Write(a.Listener); chk.E(err) {
		// 	return
		// }
	}

}
