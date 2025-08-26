package filter

import (
	"math"
	"testing"

	"lukechampine.com/frand"
	"next.orly.dev/pkg/crypto/ec/schnorr"
	"next.orly.dev/pkg/crypto/ec/secp256k1"
	"next.orly.dev/pkg/crypto/sha256"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/utils"
	"next.orly.dev/pkg/utils/values"

	"lol.mleku.dev/chk"
)

func TestT_MarshalUnmarshal(t *testing.T) {
	var err error
	const bufLen = 4000000
	dst := make([]byte, 0, bufLen)
	dst1 := make([]byte, 0, bufLen)
	dst2 := make([]byte, 0, bufLen)
	for _ = range 20 {
		f := New()
		if f, err = GenFilter(); chk.E(err) {
			t.Fatal(err)
		}
		dst = f.Marshal(dst)
		dst1 = append(dst1, dst...)
		// now unmarshal
		var rem []byte
		fa := New()
		if rem, err = fa.Unmarshal(dst); chk.E(err) {
			t.Fatalf("unmarshal error: %v\n%s\n%s", err, dst, rem)
		}
		dst2 = fa.Marshal(nil)
		if !utils.FastEqual(dst1, dst2) {
			t.Fatalf("marshal error: %v\n%s\n%s", err, dst1, dst2)
		}
		dst, dst1, dst2 = dst[:0], dst1[:0], dst2[:0]
	}
}

// GenFilter is a testing tool to create random arbitrary filters for tests.
func GenFilter() (f *F, err error) {
	f = New()
	n := frand.Intn(16)
	for _ = range n {
		id := make([]byte, sha256.Size)
		frand.Read(id)
		f.Ids.T = append(f.Ids.T, id)
		// f.Ids.Field = append(f.Ids.Field, id)
	}
	n = frand.Intn(16)
	for _ = range n {
		f.Kinds.K = append(f.Kinds.K, kind.New(frand.Intn(math.MaxUint16)))
	}
	n = frand.Intn(16)
	for _ = range n {
		var sk *secp256k1.SecretKey
		if sk, err = secp256k1.GenerateSecretKey(); chk.E(err) {
			return
		}
		pk := sk.PubKey()
		f.Authors.T = append(f.Authors.T, schnorr.SerializePubKey(pk))
		// f.Authors.Field = append(f.Authors.Field, schnorr.SerializePubKey(pk))
	}
	a := frand.Intn(16)
	if a < n {
		n = a
	}
	for i := range n {
		p := make([]byte, 0, schnorr.PubKeyBytesLen*2)
		p = hex.EncAppend(p, f.Authors.T[i])
	}
	for b := 'a'; b <= 'z'; b++ {
		l := frand.Intn(6)
		var idb [][]byte
		for range l {
			bb := make([]byte, frand.Intn(31)+1)
			frand.Read(bb)
			id := make([]byte, 0, len(bb)*2)
			id = hex.EncAppend(id, bb)
			idb = append(idb, id)
		}
		idb = append([][]byte{{'#', byte(b)}}, idb...)
		*f.Tags = append(*f.Tags, tag.New(idb...))
		// f.Tags.F = append(f.Tags.F, tag.FromBytesSlice(idb...))
	}
	tn := int(timestamp.Now().I64())
	f.Since = &timestamp.T{int64(tn - frand.Intn(10000))}
	f.Until = timestamp.Now()
	if frand.Intn(10) > 5 {
		f.Limit = values.ToUintPointer(uint(frand.Intn(1000)))
	}
	f.Search = []byte("token search text")
	return
}
