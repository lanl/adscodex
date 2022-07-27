package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l0"
	"adscodex/l1"
	"adscodex/l2"
	"adscodex/utils"
)

var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")
var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block size")
var mdnum = flag.Int("mdnum", 0, "number of metadata blocks (0 = number of data blocks)")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")
var mdcsum = flag.String("mdcsum", "crc", "L1 metadata blocks checksum type (rs for Reed-Solomon, crc for CRC)")
var dtcsum = flag.String("dtcsum", "parity", "L1 data blocks checksum type (parity or even)")
var compat = flag.Bool("compat", false, "compatibility with 0.9")
var rndomize = flag.Bool("rndmz", false, "randomze data")
var shuffle = flag.Int("shuffle", 0, "random seed for shuffling the order of the oligos (0 disable)")
var start = flag.Uint64("addr", 0, "start address")
var tblpath = flag.String("tbl", "", "path to the tables")
var calcmfe = flag.Bool("mfe", false, "calculate minimum free energy")
var temp = flag.Float64("temp", 37, "temperature (for calculating mfe)")

func main() {
	flag.Parse()

	p5, ok := long.FromString(*p5str)
	if !ok {
		fmt.Printf("Invalid 5'-end primer\n")
		return
	}

	p3, ok := long.FromString(*p3str)
	if !ok {
		fmt.Printf("Invalid 3'-end primer\n")
		return
	}

	if *tblpath != "" {
		l0.SetLookupTablePath(*tblpath)
	}

	cdc, err := l2.NewCodec(p5, p3, *dbnum, *mdsz, *mdnum, *mdcnum, *dseqnum, *rseqnum)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	cdc.SetCompat(*compat)

	if flag.NArg() != 1 {
		fmt.Printf("Expecting file name\n");
		return
	}

	var mc, dc int
	switch  *mdcsum {
	default:
		fmt.Printf("Invalid metadata checksum type\n")
		return

	case "rs":
		mc = l1.CSumRS

	case "crc":
		mc = l1.CSumCRC
	}

	if err := cdc.SetMetadataChecksum(mc); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	switch  *dtcsum {
	default:
		fmt.Printf("Invalid data checksum type: %s\n", *dtcsum)
		return

	case "parity":
		dc = l1.CSumParity

	case "even":
		dc = l1.CSumEven
	}

	if err := cdc.SetDataChecksum(dc); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	cdc.SetRandomize(*rndomize)

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Printf("Error opening the file: %v\n", err)
		return
	}
	defer f.Close()

	st, _ := f.Stat()
	data := make([]byte, st.Size())
	for b := data; len(b) != 0; {
		n, err := f.Read(data)
		if err != nil {
			fmt.Printf("Error while reading the file: %v\n", err)
			return
		}

		b = b[n:]
	}

	lastaddr, oligos, err := cdc.Encode(*start, data)
	if err != nil {
		fmt.Printf("Error while encoding: %v\n", err)
		return
	}

	if *shuffle != 0 {
		rand.Seed(int64(*shuffle))
		rand.Shuffle(len(oligos),  func (i, j int) {
			oligos[i], oligos[j] = oligos[j], oligos[i]
		})
	}

	fmt.Fprintf(os.Stderr, "Address: %v::%v\n", *start, lastaddr)

	var mfemap map[oligo.Oligo]float32
	minmfe := float32(math.MaxFloat32)
	maxmfe := float32(-math.MaxFloat32)
	avgmfe := float32(0)
	if *calcmfe {
		mfemap, err = utils.CalculateMfe(oligos, float32(*temp))
		for _, ol := range oligos {
			mfe := mfemap[ol]
			if mfe < minmfe {
				minmfe = mfe
			}

			if mfe > maxmfe {
				maxmfe = mfe
			}

			avgmfe += mfe
		}

		avgmfe /= float32(len(oligos))
	}

	for i, ol := range oligos {
		if *calcmfe {
			fmt.Printf("%v,L%d,%f\n", ol, uint64(i) + *start, mfemap[ol])
		} else {
			fmt.Printf("%v,L%d\n", ol, uint64(i) + *start)
		}
	}

	if *calcmfe {
		fmt.Fprintf(os.Stderr, "mfe: min %v max %v avg %v\n", minmfe, maxmfe, avgmfe)
	}
}
