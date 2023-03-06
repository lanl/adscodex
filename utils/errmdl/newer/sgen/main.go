package main

import (
_	"errors"
	"flag"
	"fmt"
	"os"
	"time"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/utils/errmdl/newer"
)

var num = flag.Int("n", 0, "number of sequences to generate")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var dsfname = flag.String("ds", "", "synthesis dataset")
var mdfname = flag.String("md", "", "model json description")
var ierr = flag.Float64("ierr", 0, "insertion error per position (percent)")
var derr = flag.Float64("derr", 0, "deletion error per position (percent)")
var serr = flag.Float64("serr", 0, "substitution error per position (percent)")


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

	if *mdfname == "" {
		fmt.Printf("Expecting model description\n")
		return
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}

	em, err := newer.FromJson(*mdfname, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	em.Scale(*ierr/100, *serr/100, *derr/100)
//	fmt.Fprintf(os.Stderr, "%v\n", em)
//	return

	// read the dataset
	fmt.Fprintf(os.Stderr, "Reading the dataset...")
	dspool, err := utils.ReadPool([]string { *dsfname }, false, csv.Parse)
	fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	oligos := dspool.Oligos()

	if *ierr + *derr + *serr > 100 {
		fmt.Fprintf(os.Stderr, "Total error rate can't be more than 100%%\n")
		return
	}

//	errmdl := newer.New(*ierr/100, *derr/100, *serr/100, *prob, s)
	rols, nerr := em.GenMany(*num, utils.ToOligoArray(oligos))
	for _, o := range rols {
		fmt.Printf("%v\n", o)
	}

	fmt.Fprintf(os.Stderr, "%d reads, avg. errors %v\n", len(rols), float64(nerr)/float64(len(rols)))
	return
}
