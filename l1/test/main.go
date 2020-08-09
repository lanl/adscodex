package main

import (
	"flag"
	"fmt"
_	"os"
	"math/rand"
	"runtime"
_	"time"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/l1"
	"acoma/criteria"
)

var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block sizee")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "rs", "metadata error detection type (rs or crc)")
var dtctype = flag.String("dtctype", "parity", "data error detection type (parity or even)")
var iternum = flag.Int("iternum", 1000, "number of iterations")
var errate = flag.Float64("err", 1.0, "error rate (percent)")
var dfclty =  flag.Int("dfclty", 0, "decoding difficulty level")
var crit = flag.String("crit", "h4g2", "criteria")
var seed = flag.Int64("s", 0, "random generator seed")

type Stat struct {
	count	int		// number of tests
	mderr	int		// number of metadata errors
	mdfp	int		// number of metadata false positive errors
	dterr	int		// number of data errors
	dtfp	int		// number of data false positive errors
	errnum	int		// number of errors introduced in the oligos
}
	
var cdc *l1.Codec
var p5, p3 oligo.Oligo

func main() {
	var total Stat

	flag.Parse()
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
	}

	fmt.Printf("%d\n", total.count)
	fmt.Printf("%d %d %d %v %v %v %v %v %v %v\n", *dbnum, *mdsz, *mdcnum, *mdctype, *errate,
		float64(total.mderr)/float64(total.count), 
		float64(total.mdfp)/float64(total.count), 
		float64(total.dterr)/float64(*dbnum * total.count),
		float64(total.dtfp)/float64(*dbnum * total.count),
		float64(total.errnum)/float64(total.count))
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

	return err
}

func runtest(rseed int64, niter int, ch chan Stat) {
	var st Stat

	rnd := rand.New(rand.NewSource(rseed))

	blks := make([][]byte, cdc.BlockNum())
	for i := 0; i < len(blks); i++ {
		blks[i] = make([]byte, cdc.BlockSize())
	}

	for n := 0; n < niter; n++ {
		for i := 0; i < len(blks); i++ {
			for j := 0; j < len(blks[i]); j++ {
				blks[i][j] = byte(rand.Intn(256))
			}
		}

		addr := uint64(rand.Intn(int(cdc.MaxAddr() - 2)))
		ec := n%2 == 0
		ol, err := cdc.Encode(p5, p3, addr, ec, blks)
		if err != nil {
			panic(fmt.Sprintf("error while encoding: %v\n", err))
		}

		// add some errors
		seq := ol.String()
		for i := 0; i < len(seq); i++ {
			p := rnd.Float32() * 100
			if float64(p) >= *errate {
				continue
			}

			switch rnd.Intn(3) {
			case 0:
				// delete
				if i+1 < len(seq) {
					seq = seq[0:i] + seq[i+1:]
				} else {
					seq = seq[0:i]
				}
				i--

			case 1:
				// insert
				seq = seq[0:i] + oligo.Nt2String(rnd.Intn(4)) + seq[i:]
				i++

			case 2:
				// replace
				var r string

				if i+1 < len(seq) {
					r = seq[i+1:]
				}

				seq = seq[0:i] + oligo.Nt2String(rnd.Intn(4)) + r
			}

			st.errnum++
		}

		eol, _ := long.FromString(seq)
		
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

	st.count = niter
	ch <- st
}
