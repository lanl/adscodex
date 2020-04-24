package main

import (
	"flag"
	"fmt"
	"os"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/l2"
	"acoma/criteria"
)

var enctbl = flag.String("etbl", "../tbl/encnt17b13.tbl", "encoding lookup table")
var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")
var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")

func main() {
	flag.Parse()

	if *enctbl != "" {
		err := l0.LoadEncodeTable("../tblgen/encnt17b13.tbl", criteria.H4G2)
		if err != nil {
			fmt.Printf("error while loading encoding table:%s: %v\n", err)
			return
		}
	}

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

	cdc := l2.NewCodec(p5, p3, 5, 4, 2, *dseqnum, *rseqnum)
	if flag.NArg() != 1 {
		fmt.Printf("Expecting file name\n");
		return
	}
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

	_, oligos, err := cdc.Encode(0, data)
	fmt.Printf("\n")
	if err != nil {
		fmt.Printf("Error while encoding: %v\n", err)
		return
	}

	for _, ol := range oligos {
		fmt.Printf("%v\n", ol)
	}
}
