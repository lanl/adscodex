package l0

import (
	"fmt"
	"math/rand"
	"testing"
	"adscodex/oligo/short"
_	"adscodex/criteria"
)

func testEncode(t *testing.T, c *Codec) {
	pfxLen := c.PrefixLen()
//	oligoLen := c.OligoLen()
	maxVal := c.MaxVal()
	for i := 0; i < *iternum; {
		prefix := randomOligo(pfxLen)
//		if !crit.Check(prefix) {
//			continue
//		}

		val := uint64(rand.Int63n(int64(maxVal)))

		o1, err := c.Encode(prefix, val)
		if err != nil {
			t.Fatalf("encoding failed: %v", err)
		}

		va, err := c.Decode(prefix, o1)
		if err != nil {
			t.Fatalf("decoding failed: %v", err)
		}

		if val != uint64(va[0].val) {
			pfx, _ := short.Copy(prefix)
			o, _ := short.Copy(o1)
			fmt.Printf("decode doesn't match: prefix %v (%d): %v (%d): %d %d\n", pfx, pfx.Uint64(), o, o.Uint64(), val, va[0].val)
			for i := 0; i < len(va); i++ {
				v := &va[i]
				fmt.Printf("\t%d %v %v\n", v.val, &v.ol, v.prob)
			}

			fmt.Printf("Encode table for %v\n%v\n", pfx, c.EncodeTable(pfx.Uint64()))
			fmt.Printf("Decode table for %v\n%v\n", pfx, c.DecodeTable(pfx.Uint64()).String(c.OligoLen()))
			t.Fatalf("decode doesn't match: %d %d\n", val, va[0].val)
		}

		i++
	}
}

func TestEncode(t *testing.T) {
	fmt.Printf("Test encode\n")
	testEncode(t, codec)
}
