package main

import (
	"flag"
	"fmt"
_	"os"
	"math/rand"
	"runtime"
	"sync/atomic"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l0"
	"adscodex/criteria"
	"adscodex/utils/errmdl"
	"adscodex/utils/errmdl/simple"
)

var iternum = flag.Int("iternum", 1000, "number of iterations")
var ierrate = flag.Float64("ierr", 1.0, "insertion error rate (percent)")
var derrate = flag.Float64("derr", 1.0, "deletion error rate (percent)")
var serrate = flag.Float64("serr", 1.0, "substituion error rate (percent)")
var crit = flag.String("crit", "h4g2", "criteria")
var seed = flag.Int64("s", 0, "random generator seed")
var hdr = flag.Bool("hdr", false, "print the header and exit")
var prefix = flag.String("p", "CGTA", "prefix of the encoded data")
var fnum = flag.Int("fnum", 5, "number of fields")
var flen = flag.Int("flen", 10, "field length")
var mindist = flag.Int("mindist", 4, "minimum distance")
var steps = flag.Int64("maxtime", 1000000, "maximum time to spend decoding one sequence (ms)")

var pfx oligo.Oligo
var rndseed int64
var grp *l0.Group
var maxvals []int
var total uint64
var correct []uint64
var errnum uint64
var avgtime float64
var em errmdl.GenErrMdl

func main() {

	flag.Parse()
	if *hdr {
		// make sure it's the same as the Printf below
		fmt.Printf("# fldlen mindist number-of-fields errrate steps total correct avg-time(ms)\n")
		return
	}

	if err := initTest(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	nprocs := runtime.NumCPU()
	ch := make(chan bool)
	retch := make(chan float64)
	for i := 0; i < nprocs; i++ {
		go runtest(*seed + int64(i), ch, retch)
	}

	for i := 0; i < *iternum + nprocs; i++ {
		if i < *iternum {
			ch <- true
		} else {
			ch <- false
			avgtime += <- retch
		}
	}

	fmt.Printf("%d %d %d %v %d %d %v ", *flen, *mindist, *fnum, *ierrate+*derrate+*serrate, *steps, total, avgtime / float64(total))
	for _, c := range correct {
		fmt.Printf("%d ", c)
	}
	fmt.Printf("\n")
}

func initTest() error {
	if grp != nil {
		return nil
	}

	l0.SetLookupTablePath("../../tbl")
	c := criteria.Find(*crit)
	if c == nil {
		return fmt.Errorf("criteria '%s' not found\n", *crit)
	}

	lt, err := l0.LoadOrGenerateTable(c, *flen, *mindist)
	if err != nil {
		return fmt.Errorf("can't load lookup table: %v\n", err)
	}

	lts := make([]*l0.LookupTable, *fnum)
	maxvals = make([]int, *fnum)
	correct = make([]uint64, *fnum)
	for i := 0; i < len(lts); i++ {
		lts[i] = lt
		maxvals[i] = int(lt.MaxVal())
	}

	pfx, _ = long.FromString(*prefix)
	grp, err = l0.NewGroup(pfx, lts, *steps)
	if err != nil {
		return err
	}
	
	if *seed == 0 {
		rndseed = time.Now().UnixNano()
	} else {
		rndseed = *seed
	}

	em = simple.New(*ierrate/100, *derrate/100, *serrate/100, 0.8, rndseed)
	return err
}

func runtest(rseed int64, ch chan bool, retch chan float64) {
	rnd := rand.New(rand.NewSource(rndseed))
	flds := make([]int, *fnum)
	t := time.Now()
	for <- ch {
		for i := 0; i < len(flds); i++ {
			flds[i] = int(rnd.Intn(maxvals[i]))
		}

		ol, err := grp.Encode(flds)
		if err != nil {
			panic(fmt.Sprintf("error while encoding: %v\n", err))
		}

//		fmt.Printf("enc %v\n", ol)
		eol, en := em.GenOne(ol)
		atomic.AddUint64(&errnum, uint64(en))
		
		dvals, err := grp.Decode(pfx, eol)
		atomic.AddUint64(&total, 1)

//		fmt.Printf("%v: %v\n", flds, dvals)
		if err != nil {
			// no correct data at all
			continue
		}

		for i := 0; i < len(flds); i++ {
			if flds[i] == dvals[i] {
				atomic.AddUint64(&correct[i], 1)
			}
		}
	}

	d := time.Since(t)
	retch <- float64(d.Milliseconds())
}
