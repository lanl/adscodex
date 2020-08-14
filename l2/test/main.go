package main

import (
	"flag"
	"fmt"
_	"os"
	"math/rand"
	"runtime"
	"time"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/l1"
	"acoma/l2"
	"acoma/criteria"
)

var dseqnum = flag.Int("dseqnum", 3, "number of data oligos in an erasure group")
var eseqnum = flag.Int("rseqnum", 2, "number of erasure oligos in an erasure group")
var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block size")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "rs", "metadata error detection type (rs or crc)")
var dtctype = flag.String("dtctype", "parity", "data error detection type (parity or even)")
var iternum = flag.Int("iternum", 1000, "number of iterations")
var errate = flag.Float64("err", 1.0, "error rate (percent)")
var dfclty =  flag.Int("dfclty", 0, "decoding difficulty level")
var crit = flag.String("crit", "h4g2", "criteria")
var seed = flag.Int64("s", 0, "random generator seed")
var hdr = flag.Bool("hdr", false, "print the header and exit")
var depth = flag.Int("depth", 10, "depth")
var grpnum = flag.Int("grpnum", 1, "number of groups per iteration")

type Stat struct {
	count	int		// number of oligos
	size	uint64		// number of bytes en/decoded
	dur	int64		// time in milliseconds to test
	extra	int		// number of bytes out of range
	verfp	int		// number of false positives in verified data
	uverfp	int		// number of false positives in unverified data
	versz	uint64		// number of verified bytes
	uversz	uint64		// number of unverified bytes
	holesz	uint64		// number of missing bytes
	errnum	int		// number of errors introduced
	readnum	int		// number of reads
}

var cdc *l2.Codec

func main() {
	var total Stat

	flag.Parse()
	if *hdr {
		// make sure it's the same as the Printf below
		fmt.Printf("# number-of-data-blocks metadata-block-size metadata-ec-num metadata-type data-seq-num ec-seq-num difficulty error-rate verified-rate unverified-rate hole-rate extra-rate verified-false-positives unverified-false-positives average-errors average-time\n")
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
		total.size += st.size
		total.dur += st.dur
		total.extra += st.extra
		total.verfp += st.verfp
		total.uverfp += st.uverfp
		total.versz += st.versz
		total.uversz += st.uversz
		total.holesz += st.holesz
		total.readnum += st.readnum
		total.errnum += st.errnum
	}

	fmt.Printf("%d %d %d %v %v %v %v %v %v %v %v %v %v %v %v %v\n", *dbnum, *mdsz, *mdcnum, *mdctype, *dseqnum, *eseqnum, *dfclty, *errate,
		float64(total.versz)/float64(total.size),
		float64(total.uversz)/float64(total.size),
		float64(total.holesz)/float64(total.size),
		float64(total.extra)/float64(total.size),
		float64(total.verfp)/float64(total.size),
		float64(total.uverfp)/float64(total.size),
		float64(total.errnum)/float64(total.readnum),
		float64(total.dur)/float64(total.readnum))
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
	
	p5, _ := long.FromString("CGACATCTCGATGGCAGCAT")
	p3, _ := long.FromString("CAGTGAGCTGGCAACTTCCA")

	cdc, err = l2.NewCodec(p5, p3, *dbnum, *mdsz, *mdcnum, *dseqnum, *eseqnum)
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

	ecsz := cdc.ECGSize()
	data := make([]byte, ecsz * *grpnum)
	dpr := make([]bool, len(data))

	t := time.Now()
	for n := 0; n < niter; n++ {
		var ols []oligo.Oligo

		for i := 0; i < len(data); i++ {
			data[i] = byte(rnd.Intn(256))
		}

		addr := uint64(rand.Int63n(int64(cdc.MaxAddr() - 1)))
		addr -= addr%uint64(*dseqnum)						// make sure the oligos are aligned and are a single ECG

		for i := 0; i < *grpnum; i++ {
			gols, err := cdc.EncodeECG(addr + uint64(*dseqnum * i), data[i*ecsz:(i+1)*ecsz])
			if err != nil {
				panic(fmt.Sprintf("error while encoding: %v\n", err))
			}

			ols = append(ols, gols...)
		}

		// add some errors
		var nols []oligo.Oligo
		for i := 0; i < *depth * len(ols); i++ {
//		for i, ol := range ols {
			ol := ols[rnd.Int31n(int32(len(ols)))]
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

			nols = append(nols, eol)
			st.readnum++
		}

//		fmt.Printf("ols %v\n", ols)
		dss, _ := cdc.DecodeECG(*dfclty, nols)
//		fmt.Printf("dss %v\n", dss)

		for i := 0; i < len(dpr); i++ {
			dpr[i] = false
		}

		offset := addr * uint64((4 * *dbnum))	// TODO: fix this to use the codec provided values
		for _, ds := range dss {
			if ds.Offset < offset || ds.Offset >= offset+uint64(len(data)) {
//				fmt.Printf("Extra! %d %d\n", addr, ds.Offset)
				// the data is completely out the range
				st.extra += len(ds.Data)
				continue
			}

			idx := int(ds.Offset - offset)
			for i := 0; i < len(ds.Data); i++ {
				// shouldn't happen, but these things happen all the time
				if dpr[idx + i] {
					panic("overlapping DataExtent entries")
				}

				dpr[idx + i] = true
			}

			if ds.Verified {
//				fmt.Printf("Verified!\n")
				// check if the verified data is correct
				for i := 0; i < len(ds.Data); i++ {
					if ds.Data[i] != data[i+idx] {
						st.verfp++
					}
				}

				st.versz += uint64(len(ds.Data))
			} else {
//				fmt.Printf("Unverified!\n")
				// check if the unverified data is correct
				for i := 0; i < len(ds.Data); i++ {
					if ds.Data[i] != data[i+idx] {
						st.uverfp++
					}
				}

				st.uversz += uint64(len(ds.Data))
			}
		}

		// check for missing data
		for _, f := range dpr {
			if !f {
				st.holesz++
			}
		}

		st.size += uint64(len(data))
	}

	d := time.Since(t)
	st.count = niter * (*dseqnum + *eseqnum)
	st.dur = d.Milliseconds()
	ch <- st
}
