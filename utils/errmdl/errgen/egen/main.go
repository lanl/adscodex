package main

import (
_	"errors"
	"flag"
	"fmt"
	"os"
	"time"
	"acoma/io/csv"
	"acoma/utils"
	"acoma/utils/errmdl/errgen"
)

var num = flag.Int("n", 0, "number of sequences to generate")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var dsfname = flag.String("ds", "", "synthesis dataset")


func main() {
	flag.Parse()

	if *num == 0 {
		fmt.Fprintf(os.Stderr, "Expecting number of sequences to generate\n")
		return
	}

	if *dsfname == "" {
		fmt.Fprintf(os.Stderr, "Expecting dataset file name\n")
		return
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Expecting match map name\n")
		return
	}

	// read the dataset
	dspool, err := utils.ReadPool([]string { *dsfname }, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	oligos := dspool.Oligos()

	errmdl, err := errgen.New(flag.Arg(0), s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	rols, nerr := errmdl.GenMany(*num, utils.ToOligoArray(oligos))
	for _, o := range rols {
		fmt.Printf("%v\n", o)
	}

	fmt.Fprintf(os.Stderr, "%d reads, avg. errors %v\n", len(rols), float64(nerr)/float64(len(rols)))
	return
}
