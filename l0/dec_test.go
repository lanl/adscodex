package l0

import (
	"fmt"
	"testing"
_	"adscodex/oligo/short"
_	"adscodex/criteria"
)

func testDecode(t *testing.T, c *Codec) {
	pfxLen := c.PrefixLen()
	oligoLen := c.OligoLen()
//	maxVal := c.MaxVal()
	for i := 0; i < *iternum; {
		prefix := randomOligo(pfxLen)
//		if !crit.Check(prefix) {
//			continue
//		}

		o := randomOligo(oligoLen)
		va, err := c.Decode(prefix, o)
		if err != nil {
			t.Fatalf("decoding failed: %v", err)
		}

		if len(va) == 0 {
			fmt.Printf("%v null\n", o)
		}

//		fmt.Printf("*** %v %d\n", o, va[0].val)
		i++
	}
}


func TestDecode(t *testing.T) {
	fmt.Printf("Test decode\n")
	testDecode(t, codec)
}
