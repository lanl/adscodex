package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"adscodex/criteria"
	"adscodex/oligo/long"
	"adscodex/l0"
	"adscodex/l1"
	"adscodex/l2"
)

var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")

var dbnum = flag.Int("dbnum", 9, "number of data blocks")
var dbsz = flag.Int("dbsz", 10, "size of a data block in nts")
var dbmindist = flag.Int("dbmindist", 4, "minimum distance between oligos in data blocks")

var mdnum = flag.Int("mdnum", 4, "number of metadata blocks")
var mdsz = flag.Int("mdsz", 10, "metadata block size in nts")
var mdmindist = flag.Int("mdmindist", 4, "minimum distance between oligos in metadata blocks")
var mdcnum = flag.Int("mdcnum", 1, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")

var maxtime = flag.Int64("maxtime", 1000, "maximumm time (in ms) to spend decoding a sequence")

var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")

var rndomize = flag.Bool("rndmz", false, "randomze data")
var shuffle = flag.Int("shuffle", 0, "random seed for shuffling the order of the oligos (0 disable)")
var start = flag.Uint64("addr", 0, "start address")
var tblpath = flag.String("tbl", "", "path to the tables")

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

	cdc, err := l2.NewCodec(p5, p3, *dbnum, *dbsz, *dbmindist, *mdnum, *mdsz, *mdmindist, *mdcnum, *dseqnum, *rseqnum, *maxtime, criteria.H4G2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if flag.NArg() != 1 {
		fmt.Printf("Expecting file name\n");
		return
	}

	var mc int
	switch  *mdctype {
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
	for i, ol := range oligos {
		fmt.Printf("%v,L%d\n", ol, uint64(i) + *start)
	}
}
