package event

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"lukechampine.com/frand"
	"next.orly.dev/pkg/encoders/event/examples"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/bufpool"
	"next.orly.dev/pkg/utils/units"
)

func TestMarshalJSONUnmarshalJSON(t *testing.T) {
	for range 10000 {
		ev := New()
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
		ev.Content = []byte(`some text content

	with line breaks and tabs and other stuff
`)
		ev.Sig = frand.Bytes(64)
		// log.I.S(ev)
		// b, err := ev.MarshalJSON()
		var err error
		var b []byte
		if b, err = json.Marshal(ev); chk.E(err) {
			t.Fatal(err)
		}
		var bc []byte
		bc = append(bc, b...)
		ev2 := New()
		if err = json.Unmarshal(b, ev2); chk.E(err) {
			t.Fatal(err)
		}
		var b2 []byte
		if b2, err = json.Marshal(ev2); err != nil {
			t.Fatal(err)
		}
		if !utils.FastEqual(bc, b2) {
			t.Errorf("failed to re-marshal back original")
		}
		// free up the resources for the next iteration
		ev.Free()
		ev2.Free()
		bufpool.PutBytes(b)
		bufpool.PutBytes(b2)
		bufpool.PutBytes(bc)
	}
}

func TestExamplesCache(t *testing.T) {
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	scanner.Buffer(make([]byte, 0, 4*units.Mb), 4*units.Mb)
	var err error
	for scanner.Scan() {
		b := scanner.Bytes()
		c := bufpool.Get()
		c = c[:0]
		c = append(c, b...)
		ev := New()
		if err = json.Unmarshal(b, ev); chk.E(err) {
			t.Fatal(err)
		}
		var b2 []byte
		// can't use json.Marshal as it improperly escapes <, > and &.
		if b2, err = ev.MarshalJSON(); err != nil {
			t.Fatal(err)
		}
		if !utils.FastEqual(c, b2) {
			log.I.F("\n%s\n%s", c, b2)
			t.Fatalf("failed to re-marshal back original")
		}
		ev.Free()
		// Don't return scanner.Bytes() to the pool as it's not a buffer we own
		// bufpool.PutBytes(b)
		bufpool.PutBytes(b2)
		bufpool.PutBytes(c)
	}
}
