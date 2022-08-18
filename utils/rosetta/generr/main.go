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
	"adscodex/oligo/long"
_	"time"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/utils/errmdl/simple"
)

var onum = flag.Int("n", 0, "number of sequences to generate")
var depth = flag.Int("d", 1, "depth")
//var olen = flag.Int("olen", 100, "oligo length")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var ierr = flag.Float64("ierr", 0.1, "insertion error per position (percent)")
var derr = flag.Float64("derr", 0.1, "deletion error per position (percent)")
var serr = flag.Float64("serr", 0.1, "substitution error per position (percent)")
var fnum = flag.Int("f", 2, "fragment number")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func genRand(r *rand.Rand, olen int) (ret oligo.Oligo) {
	ret = long.New(olen)
	for n := 0; n < olen; {
		v := r.Uint64()
		for i := 0; i < 16 && n < olen; i++ {
			ret.Set(n, int(v&3))
			v >>= 2
			n++
		}
	}

	return
}

func main() {
	flag.Parse()

	if *onum == 0 {
		fmt.Fprintf(os.Stderr, "Expecting number of sequences to generate\n")
		return
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Expecting oligo file(s)\n")
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
	var total uint64
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

	var progress uint64
	correct := make([]uint64, *fnum)
	avgcnt := make([]uint64, *fnum)
	avgmatch := make([]uint64, *fnum)

	for i := 0; i < procnum; i++ {
		go func(id int) {
			pseed := int64(id*2) + (int64(*seed)<<16)
			errmdl := simple.New(*ierr/100, *derr/100, *serr/100, 0.8, pseed + 1)

			olg := make([]oligo.Oligo, *fnum)
			for i := 0; i < ol4proc; i++ {
				olg[0] = ols[0][rand.Intn(len(ols))]
				ol := olg[0].Clone()
				for n := 1; n < *fnum; n++ {
					olg[n] = ols[n][rand.Intn(len(ols[n]))]
					ol.Append(olg[n])
				}

				olmap := make([]map[oligo.Oligo]int, *fnum)
				for n := 0; n < *fnum; n++ {
					olmap[n] = make(map[oligo.Oligo]int)
				}

				for d := 0; d < *depth; d++ {
					eol, _ := errmdl.GenOne(ol)

					if atomic.AddUint64(&progress, 1) % 10000 == 0 {
						fmt.Fprintf(os.Stderr, ".")
					}

					eolen := eol.Len()
					start := 0
					for n := 0; n < *fnum; n++ {
						eo := eol.Slice(start, start+olen[n])
						m := pools[n].SearchMin(eo)

						if m != nil {
							olmap[n][m.Seq]++
						}

						start += (eolen * olen[n]) / tolen
						if (eolen * olen[n]) % tolen > tolen/2 {
							start++
						}

//						if n+1 < *fnum && start + olen[n+1] > eolen {
//							start = eolen - olen[n+1]
//						}
					}
				}

				for n := 0; n < *fnum; n++ {
					atomic.AddUint64(&avgmatch[n], uint64(len(olmap[n])))

					var eol oligo.Oligo
					m := -1
					for v, i := range olmap[n] {
						if i > m {
							m = i
							eol = v
						}
					}

					if eol == olg[n] {
						atomic.AddUint64(&correct[n], 1)
					}

					atomic.AddUint64(&avgcnt[n], uint64(m))
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

	fmt.Printf("\n")
	for n := 0; n < *fnum; n++ {
		fmt.Printf("Field %d: incorrect %g (%d/%d) matches %v count %v\n", n, float64(total-correct[n])/float64(total), total - correct[n], total, avgcnt[n]/total, avgmatch[n]/total)
	}
}
