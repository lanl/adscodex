package main

import (
_	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
_	"os"
	"runtime"
	"sync/atomic"
	"adscodex/oligo"
	"adscodex/oligo/long"
_	"time"
_	"adscodex/io/csv"
_	"adscodex/utils"
	"adscodex/utils/errmdl/simple"
)

var onum = flag.Int("n", 0, "number of sequences to generate")
var depth = flag.Int("d", 1, "depth")
var olen = flag.Int("olen", 100, "oligo length")
var seed = flag.Int64("seed", 0, "seed for the random generator used for the data")
var ierr = flag.Float64("ierr", 0.1, "insertion error per position (percent)")
var derr = flag.Float64("derr", 0.1, "deletion error per position (percent)")
var serr = flag.Float64("serr", 0.1, "substitution error per position (percent)")

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
		fmt.Printf("Expecting number of sequences to generate\n")
		return
	}

	var total uint64

	procnum := runtime.NumCPU()
	ol4proc := *onum / procnum + 1
	errhist := make([]map[int]int, procnum)
	disthist := make([]map[int]int, procnum)
	ch := make(chan bool)
	for i := 0; i < procnum; i++ {
		go func(id int) {
			pseed := int64(id*2) + (int64(*seed)<<16)
			rnd := rand.New(rand.NewSource(pseed))
			errmdl := simple.New(*ierr/100, *derr/100, *serr/100, 0.8, pseed + 1)
			errhist[id] = make(map[int]int)
			disthist[id] = make(map[int]int)

			ehist := errhist[id]
			dhist := disthist[id]
			for i := 0; i < ol4proc; {
				ol := genRand(rnd, *olen)
				emin := math.MaxInt
				dmin := math.MaxInt
				var j int
				for j = 0; j < *depth && i < ol4proc; j++ {
					eol, enum := errmdl.GenOne(ol)
					if emin > enum {
						emin = enum
					}

					dist, _ := oligo.Diff(ol, eol)
					if dmin > dist {
						dmin = dist
					}
					atomic.AddUint64(&total, 1)
					i++
				}

				if emin != math.MaxInt {
					ehist[emin] += j
				}

				
				if dmin != math.MaxInt {
					dhist[dmin] += j
				}
			}

			ch <- true
			
		}(i)
	}

	// wait for all procs to finish
	for i := 0; i < procnum; i++ {
		<- ch
	}

	// collect all stats
	ehist := make(map[int]int)
	for _, m := range errhist {
		for k, v := range m {
			ehist[k] += v
		}
	}

	dhist := make(map[int]int)
	for _, m := range disthist {
		for k, v := range m {
			dhist[k] += v
		}
	}

	var ecum, dcum float64
	for i := 0; i < *olen; i++ {
		e := float64(ehist[i])/float64(total)
		d := float64(dhist[i])/float64(total)
		ecum += e
		dcum += d
		fmt.Printf("%v %v %v %v %v\n", i, e, d, ecum, dcum)
	}
}
