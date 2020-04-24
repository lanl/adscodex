package l0

import (
	"math/rand"
	"testing"
	"acoma/oligo/short"
	"acoma/criteria"
)

func testEncode(t *testing.T, olen int, crit criteria.Criteria) {
	for i := 0; i < *iternum; {
		prefix := randomOligo(4)
		if !crit.Check(prefix) {
			continue
		}

		val := uint64(rand.Int63n(1<<(2*olen - 2)))

		o1, err := Encode(prefix, val, olen, criteria.H4G2)
		if err != nil {
			t.Fatalf("encoding failed: %v", err)
		}

		if etbl {
			o2, err := encodeSlow(prefix, short.New(olen), val, crit)
			if err != nil {
				t.Fatalf("encoding slow failed: %v", err)
			}

			if o1.Cmp(o2) != 0 {
				t.Fatalf("slow and fast encode not the same: %v %v", o1, o2)
			}
		}

		val2, err := Decode(prefix, o1, crit)
		if err != nil {
			t.Fatalf("decoding failed: %v", err)
		}

		if val != val2 {
			t.Fatalf("decode doesn't match: %d %d\n", val, val2)
		}

		i++
	}
}

func TestEncode(t *testing.T) {
	testEncode(t, 4, criteria.H4G2)
//	testEncode(t, 17)
}
