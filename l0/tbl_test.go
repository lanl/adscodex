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
_	"adscodex/errmdl/moderate"
)

var iternum = flag.Int("n", 5, "number of iterations")
var tblname = flag.String("d", "../tbl/165o5b8.tbl", "lookup table")

// Agilent HiFi
var ins165 = []float64 { 4.403698131076149e-05, 2.6212501802815333e-05, 2.926767698912052e-05, 7.256516148030716e-05, }
var del165 = []float64 { 0.0006892946500215952, 0.000584897462448752, 0.0005657541593176433, 0.0005124848696955377, }
var sub165 = [][]float64 {
        { 0.9987276144174138, 0.000818229475771005, 0.00019609499909436324, 0.00025806110772090925,  },
        { 0.0003704453453594607, 0.9992087823325114, 0.00021030220544239006, 0.00021047011668671566,  },
        { 0.0005570297030248491, 0.0003686210440884982, 0.9988293102868337, 0.0002450389660528837,  },
        { 0.00030570062071379016, 0.0009591949753494968, 0.00016895939881014508, 0.9985661450051265,  },
}

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
//	emdl := moderate.New(ins165, del165, sub165)
//	codec, err = New(5, 256, 4, 0.0000001, criteria.H4G2, emdl)
	if err != nil {
		fmt.Printf("Load error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}
