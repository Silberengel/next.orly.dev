package main

import (
	"time"

	"github.com/pkg/profile"
	lol "lol.mleku.dev"
	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/bufpool"
)

func main() {
	lol.SetLogLevel("info")
	prof := profile.Start(profile.CPUProfile)
	defer prof.Stop()
	for range 1000000 {
		ev := event.New()
		ev.ID = frand.Bytes(32)
		ev.Pubkey = frand.Bytes(32)
		ev.CreatedAt = time.Now().Unix()
		ev.Kind = 1
		ev.Tags = &tag.S{
			{T: [][]byte{[]byte("t"), []byte("hashtag")}},
			{
				T: [][]byte{
					[]byte("e"),
					hex.EncAppend(nil, frand.Bytes(32)),
				},
			},
		}
		ev.Content = frand.Bytes(frand.Intn(1024) + 1)
		ev.Sig = frand.Bytes(64)
		// log.I.S(ev)
		b, err := ev.MarshalJSON()
		if chk.E(err) {
			return
		}
		var bc []byte
		bc = append(bc, b...)
		// log.I.F("%s", bc)
		ev2 := event.New()
		if err = ev2.UnmarshalJSON(b); chk.E(err) {
			return
		}
		var b2 []byte
		if b2, err = ev.MarshalJSON(); err != nil {
			return
		}
		if !utils.FastEqual(bc, b2) {
			return
		}
		// free up the resources for the next iteration
		ev.Free()
		ev2.Free()
		bufpool.PutBytes(b)
		bufpool.PutBytes(b2)
		bufpool.PutBytes(bc)
	}
}
