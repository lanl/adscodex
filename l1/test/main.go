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
	"adscodex/l0"
	"adscodex/l1"
_	"adscodex/criteria"
	"adscodex/errmdl/simple"
)

var dbnum = flag.Int("dbnum", 7, "number of data blocks")
var mdnum = flag.Int("mdnum", 3, "metadata block sizee")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")
var iternum = flag.Int("iternum", 1000, "number of iterations")
var ierrate = flag.Float64("ierr", 1.0, "insertion error rate (percent)")
var derrate = flag.Float64("derr", 1.0, "deletion error rate (percent)")
var serrate = flag.Float64("serr", 1.0, "substituion error rate (percent)")
var dfclty =  flag.Int("dfclty", 0, "decoding difficulty level")
var tbl = flag.String("tbl", "../../tbl/165o6b8.tbl", "codec table")
var seed = flag.Int64("s", 0, "random generator seed")
var hdr = flag.Bool("hdr", false, "print the header and exit")

type Stat struct {
	count	int		// number of tests
	mderr	int		// number of metadata errors
	mdfp	int		// number of metadata false positive errors
	dterr	int		// number of data errors
	dtfp	int		// number of data false positive errors
	errnum	int		// number of errors introduced in the oligos
	dur	int64		// time in milliseconds to test
}

var cdc *l1.Codec
var p5, p3 oligo.Oligo
var em *simple.SimpleErrorModel
var rndseed int64

func main() {
	var total Stat

	flag.Parse()
	if *hdr {
		// make sure it's the same as the Printf below
		fmt.Printf("# number-of-data-blocks metadata-block-size metadata-ec-num metadata-type difficulty error-rate metadata-errors metadata-false-positive data-errors data-false-positives average-errors average-time(ms)\n")
		return
	}

	if err := initTest(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	nprocs := runtime.NumCPU()
	ch := make(chan Stat)
	for i := 0; i < nprocs; i++ {
		go runtest(*seed + int64(i), *iternum / nprocs, ch)
	}

	for i := 0; i < nprocs; i++ {
		st := <- ch
		total.count += st.count
		total.mderr += st.mderr
		total.mdfp += st.mdfp
		total.dterr += st.dterr
		total.dtfp += st.dtfp
		total.errnum += st.errnum
		total.dur += st.dur
	}

	fmt.Printf("%d %d %d %v %v %v %v %v %v %v %v %v\n", *dbnum, *mdnum, *mdcnum, *mdctype, *dfclty, *ierrate + *derrate + *serrate,
		float64(total.mderr)/float64(total.count),
		float64(total.mdfp)/float64(total.count), 
		float64(total.dterr)/float64(*dbnum * total.count),
		float64(total.dtfp)/float64(*dbnum * total.count),
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

	c0, err := l0.Load(*tbl)
	if err != nil {
		return err
	}

	cdc, err = l1.NewCodec(*dbnum, *mdnum, *mdcnum, c0)
	if err != nil {
		return err
	}
	
	switch *mdctype {
	default:
		err = fmt.Errorf("Error: invalid metadata EC type")

	case "crc":
		err = cdc.SetMetadataChecksum(l1.CSumCRC)

	case "rs":
		err = cdc.SetMetadataChecksum(l1.CSumRS)
	}

	em = simple.New(*ierrate/100, *derrate/100, *serrate/100, 0.8, *seed)
	if *seed == 0 {
		rndseed = time.Now().UnixNano()
	} else {
		rndseed = *seed
	}

	return err
}

func runtest(rseed int64, niter int, ch chan Stat) {
	var st Stat

	blks := make([]byte, cdc.DataNum())
	rnd := rand.New(rand.NewSource(rndseed))
	t := time.Now()
	for n := 0; n < niter; n++ {
		for i := 0; i < len(blks); i++ {
			blks[i] = byte(rnd.Intn(256))
		}

		addr := uint64(rnd.Intn(int(cdc.MaxAddr() - 2)))
		ec := n%2 == 0
		ol, err := cdc.Encode(p5, p3, addr, ec, blks)
		if err != nil {
			panic(fmt.Sprintf("error while encoding: %v\n", err))
		}

		eol, en := em.GenOne(ol)
		st.errnum += en
		
		daddr, dec, data, err := cdc.Decode(p5, p3, eol, *dfclty)
		if err != nil {
			st.mderr++
			st.dterr += len(blks)
			continue
		}

		if addr != daddr || ec != dec {
			st.mdfp++
		}

		for i := 0; i < len(blks); i++ {
			d := data[i]
			b := blks[i]
			if d != b {
				st.dtfp++
				break
			}
		}
	}

	d := time.Since(t)
	st.count = niter
	st.dur = d.Milliseconds()
	ch <- st
}
