package l0

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
_	"adscodex/criteria"
)

var iternum = flag.Int("n", 5, "number of iterations")
var tblname = flag.String("d", "165.tbl", "lookup table")

var codec *Codec

func randomString(l int) string {
	// don't allow oligos of 0 length
	if l == 0 {
		l = 1
	}

	so := ""
	for i := 0; i < l; i++ {
		so += oligo.Nt2String(rand.Intn(4))
	}

	return so
}

func randomOligo(l int) oligo.Oligo {
	so := randomString(l)

	// randomly return some of the oligos as short, so we can test
	// the interoperability
	if l < 31 && rand.Intn(2) == 0 {
		return short.FromString1(so)
	}
		
	return long.FromString1(so)
}

func TestMain(m *testing.M) {
	var err error

	flag.Parse()
	codec, err = Load(*tblname)
	if err != nil {
		fmt.Printf("Load error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
