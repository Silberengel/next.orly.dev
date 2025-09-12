package store

import (
	"net/http"

	"next.orly.dev/pkg/encoders/envelopes/okenvelope"
)

type Responder = http.ResponseWriter
type Req = *http.Request
type OK = okenvelope.T
