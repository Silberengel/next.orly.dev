package app

import (
	"lol.mleku.dev/log"
)

func (s *Server) HandleMessage(msg []byte) {
	log.I.F("received message:\n%s\n", msg)
}
