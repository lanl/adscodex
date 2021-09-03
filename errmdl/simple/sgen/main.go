package main

import (
_	"errors"
	"flag"
	"fmt"
	"os"
	"time"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/errmdl/simple"
)

var num = flag.Int("n", 0, "number of sequences to generate")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var dsfname = flag.String("ds", "", "synthesis dataset")
var ierr = flag.Float64("ierr", 0.1, "insertion error per position (percent)")
var derr = flag.Float64("derr", 0.1, "deletion error per position (percent)")
var serr = flag.Float64("serr", 0.1, "substitution error per position (percent)")
var prob = flag.Float64("prob", 0.8,  "probability for negative binomial distribution")


func main() {
	flag.Parse()

	if *num == 0 {
		fmt.Printf("Expecting number of sequences to generate\n")
		return
	}

	if *dsfname == "" {
		fmt.Printf("Expecting dataset file name\n")
		return
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}

	// read the dataset
	dspool, err := utils.ReadPool([]string { *dsfname }, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	oligos := dspool.Oligos()

	if *ierr + *derr + *serr > 100 {
		fmt.Fprintf(os.Stderr, "Total error rate can't be more than 100%%\n")
		return
	}

	errmdl := simple.New(*ierr/100, *derr/100, *serr/100, *prob, s)
	rols, nerr := errmdl.GenMany(*num, utils.ToOligoArray(oligos))
	for _, o := range rols {
		fmt.Printf("%v\n", o)
	}

	fmt.Fprintf(os.Stderr, "%d reads, avg. errors %v\n", len(rols), float64(nerr)/float64(len(rols)))
	return
}
