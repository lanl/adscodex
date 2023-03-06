package main

import (
_	"errors"
	"flag"
	"fmt"
_	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"sync/atomic"
	"adscodex/oligo"
_	"adscodex/oligo/long"
_	"time"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/criteria"
)

var onum = flag.Int("n", 0, "number of sequences to generate")
//var olen = flag.Int("olen", 100, "oligo length")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var fnum = flag.Int("f", 2, "fragment number")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()

	if *onum == 0 {
		fmt.Fprintf(os.Stderr, "Error: Expecting number of sequences to generate\n")
		return
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Error: Expecting oligo file(s)\n")
		return
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n")
			return
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	pools := make([]*utils.Pool, *fnum)
	ols := make([][]*utils.Oligo, *fnum)
	olen := make([]int, *fnum)
	var tolen int
	var total, goodgc, goodhp, good uint64
	for n := 0; n < *fnum; n++ {
		var err error

		if flag.NArg() <= n {
			pools[n] = pools[n-1]
		} else {
			pools[n], err = utils.ReadPool([]string{flag.Arg(n)}, false, csv.Parse)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			pools[n].InitSearch()
		}

		ols[n] = pools[n].Oligos()
		olen[n] = ols[n][0].Len()
		tolen += olen[n]
	}

	procnum := runtime.NumCPU()
	ol4proc := *onum / procnum + 1
	ch := make(chan bool)

	for i := 0; i < procnum; i++ {
		go func(id int) {
			olg := make([]oligo.Oligo, *fnum)
			for i := 0; i < ol4proc; i++ {
				olg[0] = ols[0][rand.Intn(len(ols[0]))]
				ol := olg[0].Clone()
				for n := 1; n < *fnum; n++ {
					olg[n] = ols[n][rand.Intn(len(ols[n]))]
					ol.Append(olg[n])
				}

				gc := oligo.GCcontent(ol)
				hp := criteria.H4G2.Check(ol)

				if gc >= 0.45 && gc <= 0.55 {
					atomic.AddUint64(&goodgc, 1)
				}

				if hp {
					atomic.AddUint64(&goodhp, 1)
				}

				if gc >= 0.45 && gc <= 0.55 && hp {
					atomic.AddUint64(&good, 1)
				}

				atomic.AddUint64(&total, 1)
			}

			ch <- true
			
		}(i)
	}

	// wait for all procs to finish
	for i := 0; i < procnum; i++ {
		<- ch
	}

	fmt.Printf("Good GC %v Good HP %v Good %v\n", float64(goodgc)/float64(total), float64(goodhp)/float64(total), float64(good)/float64(total))
}
