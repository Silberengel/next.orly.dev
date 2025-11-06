package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/encoders/envelopes"
	"next.orly.dev/pkg/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/encoders/envelopes/closeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/countenvelope"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/envelopes/noticeenvelope"
	"next.orly.dev/pkg/encoders/envelopes/reqenvelope"
)

// validateJSONMessage checks if a message contains invalid control characters
// that would cause JSON parsing to fail
func validateJSONMessage(msg []byte) (err error) {
	for i, b := range msg {
		// Check for invalid control characters in JSON strings
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			// Allow some control characters that might be valid in certain contexts
			// but reject form feed (\f), backspace (\b), and other problematic ones
			switch b {
			case '\b', '\f', 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
				0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F:
				return fmt.Errorf("invalid control character 0x%02X at position %d", b, i)
			}
		}
		// Check for non-printable characters that might indicate binary data
		if b > 127 && !unicode.IsPrint(rune(b)) {
			// Allow valid UTF-8 sequences, but be suspicious of random binary data
			if i < len(msg)-1 {
				// Quick check: if we see a lot of high-bit characters in sequence,
				// it might be binary data masquerading as text
				highBitCount := 0
				for j := i; j < len(msg) && j < i+10; j++ {
					if msg[j] > 127 {
						highBitCount++
					}
				}
				if highBitCount > 7 { // More than 70% high-bit chars in a 10-byte window
					return fmt.Errorf("suspicious binary data detected at position %d", i)
				}
			}
		}
	}
	return
}

func (l *Listener) HandleMessage(msg []byte, remote string) {
	// Handle blacklisted IPs - discard messages but keep connection open until timeout
	if l.isBlacklisted {
		// Check if timeout has been reached
		if time.Now().After(l.blacklistTimeout) {
			log.W.F("blacklisted IP %s timeout reached, closing connection", remote)
			// Close the connection by cancelling the context
			// The websocket handler will detect this and close the connection
			return
		}
		log.D.F("discarding message from blacklisted IP %s (timeout in %v)", remote, time.Until(l.blacklistTimeout))
		return
	}

	msgPreview := string(msg)
	if len(msgPreview) > 150 {
		msgPreview = msgPreview[:150] + "..."
	}
	// log.D.F("%s processing message (len=%d): %s", remote, len(msg), msgPreview)

	// Validate message for invalid characters before processing
	if err := validateJSONMessage(msg); err != nil {
		log.E.F("%s message validation FAILED (len=%d): %v", remote, len(msg), err)
		if noticeErr := noticeenvelope.NewFrom(fmt.Sprintf("invalid message format: contains invalid characters: %s", msg)).Write(l); noticeErr != nil {
			log.E.F("%s failed to send validation error notice: %v", remote, noticeErr)
		}
		return
	}

	l.msgCount++
	var err error
	var t string
	var rem []byte

	// Attempt to identify the envelope type
	if t, rem, err = envelopes.Identify(msg); err != nil {
		log.E.F(
			"%s envelope identification FAILED (len=%d): %v", remote, len(msg),
			err,
		)
		// Don't log message preview as it may contain binary data
		chk.E(err)
		// Send error notice to client
		if noticeErr := noticeenvelope.NewFrom("malformed message").Write(l); noticeErr != nil {
			log.E.F(
				"%s failed to send malformed message notice: %v", remote,
				noticeErr,
			)
		}
		return
	}

	log.T.F(
		"%s identified envelope type: %s (payload_len=%d)", remote, t, len(rem),
	)

	// Process the identified envelope type
	switch t {
	case eventenvelope.L:
		log.T.F("%s processing EVENT envelope", remote)
		l.eventCount++
		err = l.HandleEvent(rem)
	case reqenvelope.L:
		log.T.F("%s processing REQ envelope", remote)
		l.reqCount++
		err = l.HandleReq(rem)
	case closeenvelope.L:
		log.T.F("%s processing CLOSE envelope", remote)
		err = l.HandleClose(rem)
	case authenvelope.L:
		log.T.F("%s processing AUTH envelope", remote)
		err = l.HandleAuth(rem)
	case countenvelope.L:
		log.T.F("%s processing COUNT envelope", remote)
		err = l.HandleCount(rem)
	default:
		err = fmt.Errorf("unknown envelope type %s", t)
		log.E.F(
			"%s unknown envelope type: %s (payload_len: %d)", remote, t,
			len(rem),
		)
	}

	// Handle any processing errors
	if err != nil {
		// Don't log context cancellation errors as they're expected during shutdown
		if !strings.Contains(err.Error(), "context canceled") {
			log.E.F("%s message processing FAILED (type=%s): %v", remote, t, err)
			// Don't log message preview as it may contain binary data
 			// Send error notice to client (use generic message to avoid control chars in errors)
			noticeMsg := fmt.Sprintf("%s processing failed", t)
			if noticeErr := noticeenvelope.NewFrom(noticeMsg).Write(l); noticeErr != nil {
				log.E.F(
					"%s failed to send error notice after %s processing failure: %v",
					remote, t, noticeErr,
				)
				return
			}
			log.T.F("%s sent error notice for %s processing failure", remote, t)
		}
	} else {
		log.T.F("%s message processing SUCCESS (type=%s)", remote, t)
	}
}
