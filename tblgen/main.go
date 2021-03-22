package main

import (
	"flag"
	"fmt"
	"adscodex/l0"
	"adscodex/criteria"
)

var encfile = flag.String("e", "", "encoding table file name")
var decfile = flag.String("d", "", "decoding table file name")
var olen = flag.Int("l", 17, "oligo length")
var bits = flag.Int("b", 20, "bits in table")
var crit = flag.String("c", "h4g2", "criteria (currently only h4g2 supported)")

func main() {
	var c criteria.Criteria
	var err error

	flag.Parse()

	c = criteria.Find(*crit)
	if c == nil {
		fmt.Printf("Error: invalid criteria\n")
		return
	}

	if *encfile != "" {
		lt := l0.BuildEncodingLookupTable(4, *olen, *bits, c)
		err = lt.Write(*encfile)
		if err != nil {
			goto error
		}

		fmt.Printf("Encoding Maxvalues:\n%v\n", lt.MaxVals())
		fmt.Printf("Encoding Maxvalue:\n%v\n", lt.MaxVal())
	}

	if *decfile != "" {
		lt := l0.BuildDecodingLookupTable(4, *olen, *bits, c)
		err = lt.Write(*decfile)
		if err != nil {
			goto error
		}

		fmt.Printf("Decoding Maxvalules:\n%v\n", lt.MaxVals())
		fmt.Printf("Decoding Maxvalue:\n%v\n", lt.MaxVal())
	}

	return

error:
	fmt.Printf("Error: %v\n", err)
	return
}
