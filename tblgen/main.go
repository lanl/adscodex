package main

import (
	"flag"
	"fmt"
	"adscodex/l0"
	"adscodex/criteria"
)

var out = flag.String("o", "", "table file name")
var olen = flag.Int("olen", 10, "oligo length")
var mindist = flag.Int("mindist", 3, "minimum distance between oligos in the table")
var crit = flag.String("c", "h4g2", "criteria")
var file = flag.String("f", "", "print specified table stats")
var shuf = flag.Bool("shuffle", true, "shuffle the oligos")
var maxval = flag.Int("maxval", 0, "maximum number of oligos in the table")

func main() {
	var c criteria.Criteria
	var err error
	var lt *l0.LookupTable

	flag.Parse()

	c = criteria.Find(*crit)
	if c == nil {
		fmt.Printf("Error: invalid criteria\n")
		return
	}

	if *file != "" {
		var err error

		lt, err = l0.LoadLookupTable(*file)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("%v\n", lt)
	} else {
		fname := l0.LookupTableFilename(c, *olen, *mindist)
		if *out != "" {
			fname = *out
		}

		lt = l0.BuildLookupTable(c, *olen, *mindist, *shuf, *maxval)
		err = lt.Write(fname)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	fmt.Printf("Maxvalue: %v\n", lt.MaxVal())

	return
}
