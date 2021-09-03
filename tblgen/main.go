package main

import (
	"flag"
	"fmt"
	"adscodex/l0"
	"adscodex/criteria"
	"adscodex/errmdl"
	"adscodex/errmdl/moderate"
)

var fname = flag.String("file", "", "file name")
var olen = flag.Int("olen", 5, "oligo length")
var maxval = flag.Int("maxval", 256, "maximum value")
var crit = flag.String("c", "h4g2", "criteria (currently only h4g2 supported)")
var emdl = flag.String("e", "minion", "error model")
var minerr = flag.Float64("minerr", 0.0000001, "minimum error");

// Minion (pdist 8)
var insont = []float64 { 0.003014448720290026, 0.0038674396195582023, 0.002703262547570003, 0.00318277124022496, }
var delont = []float64 { 0.01118331719843745, 0.011193913591791711, 0.012756681443547793, 0.008576304181293657, }
var subont = [][]float64 {
	{ 0.9762083256164475, 0.005382687831843868, 0.004329306256950532, 0.014079680294758009,  },
	{ 0.0046185665595256135, 0.9737140657964496, 0.017699775627075656, 0.003967592016949176,  },
	{ 0.005256965762057342, 0.02736937814538019, 0.9643150540273302, 0.0030586020652322325,  },
	{ 0.017730218182198516, 0.0054727140356602005, 0.0028445746902455125, 0.9739524930918958,  },
}

// Agilent HiFi
var ins165 = []float64 { 4.403698131076149e-05, 2.6212501802815333e-05, 2.926767698912052e-05, 7.256516148030716e-05, }
var del165 = []float64 { 0.0006892946500215952, 0.000584897462448752, 0.0005657541593176433, 0.0005124848696955377, }
var sub165 = [][]float64 {
	{ 0.9987276144174138, 0.000818229475771005, 0.00019609499909436324, 0.00025806110772090925,  },
	{ 0.0003704453453594607, 0.9992087823325114, 0.00021030220544239006, 0.00021047011668671566,  },
	{ 0.0005570297030248491, 0.0003686210440884982, 0.9988293102868337, 0.0002450389660528837,  },
	{ 0.00030570062071379016, 0.0009591949753494968, 0.00016895939881014508, 0.9985661450051265,  },
}

func main() {
	var c criteria.Criteria
	var em errmdl.ErrMdl
	var err error

	flag.Parse()

	c = criteria.Find(*crit)
	if c == nil {
		fmt.Printf("Error: invalid criteria\n")
		return
	}

	switch *emdl {
	case "minion":
		em = moderate.New(insont, delont, subont)

	case "165":
		em = moderate.New(ins165, del165, sub165)

	default:
		fmt.Printf("Invalid error model\n")
		return
	}

	lt := l0.BuildLookupTable(*olen, *maxval, 4, *minerr, c, em)
	err = lt.Write(*fname)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
