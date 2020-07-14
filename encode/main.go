package main

import (
	"flag"
	"fmt"
	"os"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/l1"
	"acoma/l2"
	"acoma/criteria"
)

var enctbl = flag.String("etbl", "../tbl/encnt17b13.tbl", "encoding lookup table")
var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")
var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")
var mdcsum = flag.String("mdcsum", "rs", "L1 metadata blocks checksum type (rs for Reed-Solomon, crc for CRC)")
var dtcsum = flag.String("dtcsum", "parity", "L1 data blocks checksum type (parity or even)")

func main() {
	flag.Parse()

	if *enctbl != "" {
		err := l0.LoadEncodeTable(*enctbl, criteria.H4G2)
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
	if err != nil {
		fmt.Printf("Error while encoding: %v\n", err)
		return
	}

	for i, ol := range oligos {
		fmt.Printf("%v,L%d\n", ol, i)
	}
}
