package main

import (
	"flag"
	"fmt"
_	"os"
	"math/rand"
	"runtime"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
_	"adscodex/l0"
	"adscodex/l1"
	"adscodex/utils/errmdl"
	"adscodex/utils/errmdl/simple"
)

var tblname = flag.String("tbl", "../../tbl/32-10.tbl", "table name")
var maxtime = flag.Int64("maxtime", 5000, "maximumm time (in ms) to spend decoding a sequence")

var iternum = flag.Int("iternum", 1000, "number of iterations")
var ierrate = flag.Float64("ierr", 1.0, "insertion error rate (percent)")
var derrate = flag.Float64("derr", 1.0, "deletion error rate (percent)")
var serrate = flag.Float64("serr", 1.0, "substituion error rate (percent)")
var seed = flag.Int64("s", 0, "random generator seed")
var hdr = flag.Bool("hdr", false, "print the header and exit")

type Stat struct {
	count	int		// number of tests
	mderr	int		// number of metadata errors
	mdfp	int		// number of metadata false positive errors
	dterr	int		// number of data errors
	errnum	int		// number of errors introduced in the oligos
	dur	int64		// time in milliseconds to test
}

var cdc *l1.Codec
var p5, p3 oligo.Oligo
var em errmdl.GenErrMdl
var rndseed int64

func main() {
	var total Stat

	flag.Parse()
	if *hdr {
		// make sure it's the same as the Printf below
		fmt.Printf("# max-time error-rate metadata-errors metadata-false-positive data-errors  average-errors average-time(ms)\n")
		return
	}

	if err := initTest(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	nprocs := runtime.NumCPU()
	bch := make(chan bool)
	ch := make(chan Stat)
	for i := 0; i < nprocs; i++ {
		go runtest(rndseed + int64(i), bch, ch)
	}

	// run the tests
	for i := 0; i < *iternum; i++ {
		bch <- true
	}

	// tell the goroutines to exit
	for i := 0; i < nprocs; i++ {
		bch <- false
		st := <- ch
		total.count += st.count
		total.mderr += st.mderr
		total.mdfp += st.mdfp
		total.dterr += st.dterr
		total.errnum += st.errnum
		total.dur += st.dur
	}

	fmt.Printf("%d %v ", *maxtime, *ierrate + *derrate + *serrate)
	fmt.Printf("%v %v %v %v %v\n",
		float64(total.mderr)/float64(total.count),
		float64(total.mdfp)/float64(total.count), 
		float64(total.dterr)/float64(cdc.DataLen() * total.count),
		float64(total.errnum)/float64(total.count),
		float64(total.dur)/float64(total.count))
}

func initTest() error {
	var err error

	if cdc != nil {
		return nil
	}

	p5, _ = long.FromString("CGACATCTCGATGGCAGCAT")
	p3, _ = long.FromString("CAGTGAGCTGGCAACTTCCA")

	cdc, err = l1.NewCodec(p5, p3, *tblname, *maxtime)
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

func runtest(rseed int64, bch chan bool, ch chan Stat) {
	var st Stat

	data := make([]byte, cdc.DataLen())
	rnd := rand.New(rand.NewSource(rndseed))
	t := time.Now()
	n := 0
	for <-bch {
		for i := 0; i < len(data); i++ {
			data[i] = byte(rnd.Intn(256))
		}

		addr := uint64(rnd.Intn(int(cdc.MaxAddr() - 2)))
		ec := rnd.Intn(3)%2 == 0
		ol, err := cdc.Encode(addr, ec, data)
		if err != nil {
			panic(fmt.Sprintf("error while encoding: %v\n", err))
		}

		eol, en := em.GenOne(ol)
		st.errnum += en

		if eol == nil {
			panic("boo")
		}

		daddr, dec, ddata, _, err := cdc.Decode(eol)
		n++
		if err != nil {
			st.mderr++
			st.dterr += len(data)
			continue
		}

		if addr != daddr || ec != dec {
			st.mdfp++
		}

		for i := 0; i < len(data); i++ {
			if ddata[i] != data[i] {
				st.dterr++
			}
		}
	}

	d := time.Since(t)
	st.count = n
	st.dur = d.Milliseconds()
	ch <- st
}
