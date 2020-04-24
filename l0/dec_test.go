package l0

import (
	"testing"
	"acoma/oligo/short"
	"acoma/criteria"
)

func testDecode(t *testing.T, olen int, crit criteria.Criteria) {
	for i := 0; i < *iternum; {
		prefix := randomOligo(4)
		if !crit.Check(prefix) {
			continue
		}

		o := randomOligo(olen)
		val, err := Decode(prefix, o, crit)
		if err != nil {
			t.Fatalf("decoding failed: %v", err)
		}

		if dtbl {
			val2, err := decodeSlow(prefix, o, short.New(o.Len()), 0, crit)
			if err != nil {
				t.Fatalf("decoding slow failed: %v", err)
			}

			if val != val2 {
				t.Fatalf("slow and fast decode not the same: %v: %v %v", o, val, val2)
			}
		}

		o2, err := Encode(prefix, val, o.Len(), crit)
		if err != nil {
			t.Fatalf("encoding failed: %v", err)
		}

		if o.Cmp(o2) != 0 {
			t.Fatalf("encoded value doesn't match: %v %v\n", o, o2)
		}

		i++
	}
}


func TestDecode(t *testing.T) {
	testDecode(t, 4, criteria.H4G2)
}
