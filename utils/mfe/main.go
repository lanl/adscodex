package main

import (
	"flag"
	"fmt"
	"os"
	"adscodex/oligo"
	"adscodex/io/csv"
	"adscodex/utils"
)

func main() {

	flag.Parse()

	var fnames []string
	for i := 0; i < flag.NArg(); i++ {
		fnames = append(fnames, flag.Arg(i))
	}

	pool, err := utils.ReadPool(fnames, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	var ols []oligo.Oligo
	for _, ol := range pool.Oligos() {
		ols = append(ols, oligo.Oligo(ol))
	}

	mfemap, err := utils.CalculateMfe(ols, 37)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
		
	for _, ol := range pool.Oligos() {
		fmt.Printf("%v %v %v\n", ol, ol.Count(), mfemap[ol])
	}
}
