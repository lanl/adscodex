package l0

import (
	"fmt"
_	"math/rand"
	"testing"
	"adscodex/oligo"
	"adscodex/oligo/short"
_	"adscodex/criteria"
)

/*
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
*/

func TestGroup(t *testing.T) {
	var err error
	var ol oligo.Oligo
	var retvals []int
	var dist int

	fmt.Printf("Create group...")
	pfx, _ := short.FromString("ACCT")
	g, err := NewGroup(pfx, []*LookupTable{ ltable, ltable, ltable, ltable, ltable, ltable, ltable, ltable, ltable, ltable }, 100)
//	g, err := NewGroup(pfx, []*LookupTable{ ltable, ltable,  }, 1)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Done\n")

/*
	fmt.Printf("Size by depth:")
	szmap := g.triecat.SizeByDepth()
	for i := 0; i < 10000; i++ {
		d, ok := szmap[i]
		if !ok {
			break
		}

		fmt.Printf("\t%d\t%v\n", i, d)
	}
*/
	fmt.Printf("Encode...")
	vals := []int { 34, 279, 441, 22, 76, 397, 849, 3, 822, 452}
//	vals := []int { 34, 279,  }
	ol, err = g.Encode(vals)
	if err != nil {
		t.Fatalf("Encode Error: %v\n", err)
	}
	fmt.Printf("Done\n")

	fmt.Printf("encoded %v oligo: %v\n", vals, ol)

	retvals, dist, err = g.Decode(pfx, ol)
	if err != nil {
		t.Fatalf("Decode Error: %v\n", err)
	}

	fmt.Printf("decoded vals %v %v dist %d\n", retvals, ol, dist)
}
