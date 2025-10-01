package app

import (
	"fmt"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/envelopes"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/closeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/noticeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/reqenvelope"
)

func (l *Listener) HandleMessage(msg []byte, remote string) {
	msgPreview := string(msg)
	if len(msgPreview) > 150 {
		msgPreview = msgPreview[:150] + "..."
	}
	log.D.F("%s processing message (len=%d): %s", remote, len(msg), msgPreview)
	
	l.msgCount++
	var err error
	var t string
	var rem []byte
	
	// Attempt to identify the envelope type
	if t, rem, err = envelopes.Identify(msg); err != nil {
		log.E.F("%s envelope identification FAILED (len=%d): %v", remote, len(msg), err)
		log.D.F("%s malformed message content: %q", remote, msgPreview)
		chk.E(err)
		// Send error notice to client
		if noticeErr := noticeenvelope.NewFrom("malformed message: " + err.Error()).Write(l); noticeErr != nil {
			log.E.F("%s failed to send malformed message notice: %v", remote, noticeErr)
		}
		return
	}
	
	log.D.F("%s identified envelope type: %s (payload_len=%d)", remote, t, len(rem))
	
	// Process the identified envelope type
	switch t {
	case eventenvelope.L:
		log.D.F("%s processing EVENT envelope", remote)
		l.eventCount++
		err = l.HandleEvent(rem)
	case reqenvelope.L:
		log.D.F("%s processing REQ envelope", remote)
		l.reqCount++
		err = l.HandleReq(rem)
	case closeenvelope.L:
		log.D.F("%s processing CLOSE envelope", remote)
		err = l.HandleClose(rem)
	case authenvelope.L:
		log.D.F("%s processing AUTH envelope", remote)
		err = l.HandleAuth(rem)
	default:
		err = fmt.Errorf("unknown envelope type %s", t)
		log.E.F("%s unknown envelope type: %s (payload: %q)", remote, t, string(rem))
	}
	
	// Handle any processing errors
	if err != nil {
		log.E.F("%s message processing FAILED (type=%s): %v", remote, t, err)
		log.D.F("%s error context - original message: %q", remote, msgPreview)
		
		// Send error notice to client
		noticeMsg := fmt.Sprintf("%s: %s", t, err.Error())
		if noticeErr := noticeenvelope.NewFrom(noticeMsg).Write(l); noticeErr != nil {
			log.E.F("%s failed to send error notice after %s processing failure: %v", remote, t, noticeErr)
			return
		}
		log.D.F("%s sent error notice for %s processing failure", remote, t)
	} else {
		log.D.F("%s message processing SUCCESS (type=%s)", remote, t)
	}
}
