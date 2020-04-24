package main

import (
	"flag"
	"fmt"
	"acoma/l0"
	"acoma/criteria"
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
	switch *crit {
	default:
		fmt.Printf("Error: invalid criteria\n")
		return

	case "h4g2":
		c = criteria.H4G2
	}

	if *encfile != "" {
		lt := l0.BuildEncodingLookupTable(4, *olen, *bits, c)
		err = lt.Write(*encfile)
		if err != nil {
			goto error
		}
	}

	if *decfile != "" {
		lt := l0.BuildDecodingLookupTable(4, *olen, *bits, c)
		err = lt.Write(*decfile)
		if err != nil {
			goto error
		}
	}

	return

error:
	fmt.Printf("Error: %v\n", err)
	return
}
