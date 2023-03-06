package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"adscodex/oligo/long"
	"adscodex/l2"
)

var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")

var tblName = flag.String("tbl", "../tbl/32-10.tbl", "table name")
var maxtime = flag.Int64("maxtime", 1000, "maximumm time (in ms) to spend decoding a sequence")

var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")

var rndomize = flag.Bool("rndmz", false, "randomze data")
var shuffle = flag.Int("shuffle", 0, "random seed for shuffling the order of the oligos (0 disable)")
var start = flag.Uint64("addr", 0, "start address")

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

	cdc, err := l2.NewCodec(p5, p3, *tblName, *dseqnum, *rseqnum, *maxtime)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if flag.NArg() != 1 {
		fmt.Printf("Expecting file name\n");
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
	saddr := (*start * uint64((*dseqnum + *rseqnum))) / uint64(*dseqnum)
	for i, ol := range oligos {
		fmt.Printf("%v,L%d\n", ol, uint64(i) + saddr)
	}
}
