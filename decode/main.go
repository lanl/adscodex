package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/l2"
	"acoma/criteria"
)

var dectbl = flag.String("dtbl", "../tbl/decnt17b7.tbl", "decoding lookup table")
var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")
var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")

func main() {
	flag.Parse()

	if *dectbl != "" {
		err := l0.LoadDecodeTable(*dectbl, criteria.H4G2)
		if err != nil {
			fmt.Printf("error while loading decoding table:%s: %v\n", err)
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
	if flag.NArg() != 2 {
		fmt.Printf("Expecting file name\n");
		return
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Printf("Error opening the file: %v\n", err)
		return
	}
	defer f.Close()

	var oligos []oligo.Oligo
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		seq := sc.Text()
		if len(seq) == 0 {
			continue
		}

		o, ok := long.FromString(seq)
		if !ok {
			fmt.Printf("invalid sequence: %s\n", seq)
			return
		}

		oligos = append(oligos, o)
	}

	data := cdc.Decode(0, math.MaxUint64, oligos)
	of, err := os.Create(flag.Arg(1))
	if err != nil {
		fmt.Printf("Error creating the file: %s: %v\n", flag.Arg(1))
		return
	}

	for i := 0; i < len(data); i++ {
		d := &data[i]
		of.Seek(int64(d.Offset), 0)
		of.Write(d.Data)
	}
	of.Close()

	if len(data) != 1 {
		fmt.Printf("Warning: not all data was recovered, the file has holes\n")
	}
}
