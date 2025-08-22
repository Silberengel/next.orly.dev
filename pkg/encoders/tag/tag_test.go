package tag

import (
	"testing"

	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
	"next.orly.dev/pkg/utils"
)

func TestMarshalUnmarshal(t *testing.T) {
	for _ = range 1000 {
		n := frand.Intn(8)
		tg := New()
		for _ = range n {
			b1 := make([]byte, frand.Intn(8))
			_, _ = frand.Read(b1)
			tg.T = append(tg.T, b1)
		}
		tb := tg.Marshal()
		var tbc []byte
		tbc = append(tbc, tb...)
		tg2 := New()
		if _, err := tg2.Unmarshal(tb); chk.E(err) {
			t.Fatal(err)
		}
		tb2 := tg2.Marshal()
		if !utils.FastEqual(tbc, tb2) {
			t.Fatalf("failed to re-marshal back original")
		}
	}
}
