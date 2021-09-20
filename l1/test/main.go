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
	"adscodex/criteria"
	"adscodex/utils/errmdl"
	"adscodex/utils/errmdl/simple"
	"adscodex/utils/errmdl/moderate"
)

var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block sizee")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "rs", "metadata error detection type (rs or crc)")
var dtctype = flag.String("dtctype", "parity", "data error detection type (parity or even)")
var iternum = flag.Int("iternum", 1000, "number of iterations")
var ierrate = flag.Float64("ierr", 1.0, "insertion error rate (percent)")
var derrate = flag.Float64("derr", 1.0, "deletion error rate (percent)")
var serrate = flag.Float64("serr", 1.0, "substituion error rate (percent)")
var dfclty =  flag.Int("dfclty", 0, "decoding difficulty level")
var crit = flag.String("crit", "h4g2", "criteria")
var seed = flag.Int64("s", 0, "random generator seed")
var hdr = flag.Bool("hdr", false, "print the header and exit")
var emdl = flag.String("emdl", "", "error model file")
var emdlmaxerrs = flag.Int("emdlmaxerrs", 100, "filter out entries with more than the specified number of errors")
var mdrt = flag.String("moderate", "", "json file that describes a moderate error model")

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
var em errmdl.GenErrMdl
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

	fmt.Printf("%d %d %d %v %v %v %v %v %v %v %v %v\n", *dbnum, *mdsz, *mdcnum, *mdctype, *dfclty, *ierrate + *derrate + *serrate,
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

	l0.SetLookupTablePath("../../tbl")
	c := criteria.Find(*crit)
	if c == nil {
		return fmt.Errorf("criteria '%s' not found\n", *crit)
	}
	
	p5, _ = long.FromString("CGACATCTCGATGGCAGCAT")
	p3, _ = long.FromString("CAGTGAGCTGGCAACTTCCA")

	cdc, err = l1.NewCodec(*dbnum, *mdsz, *mdcnum, criteria.H4G2)
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

	switch *dtctype {
	default:
		err = fmt.Errorf("Error: invalid data EC type")

	case "parity":
		err = cdc.SetDataChecksum(l1.CSumParity)

	case "even":
		err = cdc.SetDataChecksum(l1.CSumEven)
	}

	if *emdl != "" {
		err = cdc.SetErrorModel(*emdl, *emdlmaxerrs)
	} else {
		cdc.SetSimpleErrorModel(*ierrate/100, *derrate/100, *serrate/100, *emdlmaxerrs)
	}

	if *mdrt != "" {
		em, err = moderate.FromJson(*mdrt, 0.8, *seed)
		if err != nil {
			return err
		}
	} else {
		em = simple.New(*ierrate/100, *derrate/100, *serrate/100, 0.8, *seed)
	}

	if *seed == 0 {
		rndseed = time.Now().UnixNano()
	} else {
		rndseed = *seed
	}

	return err
}

func runtest(rseed int64, niter int, ch chan Stat) {
	var st Stat

	blks := make([][]byte, cdc.BlockNum())
	for i := 0; i < len(blks); i++ {
		blks[i] = make([]byte, cdc.BlockSize())
	}

	rnd := rand.New(rand.NewSource(rndseed))
	t := time.Now()
	for n := 0; n < niter; n++ {
		for i := 0; i < len(blks); i++ {
			for j := 0; j < len(blks[i]); j++ {
				blks[i][j] = byte(rnd.Intn(256))
			}
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
			if data[i] == nil {
				st.dterr++
			} else {
				d := data[i]
				b := blks[i]
				for k := 0; k < len(d); k++ {
					if d[k] != b[k] {
						st.dtfp++
						break
					}
				}
			}
		}
	}

	d := time.Since(t)
	st.count = niter
	st.dur = d.Milliseconds()
	ch <- st
}
