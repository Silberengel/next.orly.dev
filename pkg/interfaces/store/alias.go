package store

import (
	"net/http"

	"encoders.orly/envelopes/okenvelope"
)

type Responder = http.ResponseWriter
type Req = *http.Request
type OK = okenvelope.T
