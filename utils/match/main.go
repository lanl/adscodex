package main

import (
	"flag"
	"fmt"
_	"math/rand"
	"os"
	"sync/atomic"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/io/csv"
	"adscodex/io/fastq"
	"adscodex/utils"
)

var printOligos = flag.Bool("p", true, "print oligo and reads for each match")
var datasetFile = flag.String("ds", "", "synthesis file")
var pdist = flag.Int("pdist", 3, "primer match distance")
var mdist = flag.Int("maxdist", 64, "maximum distance for matching")
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")
var ftype = flag.String("ft", "fastq", "input file type")

type Match struct {
	oligo	oligo.Oligo		// the original oligo from the synthesis file
	seq	*utils.Oligo		// the sequence from the results
	diff	string			// difference from the oligo to seq
}

func main() {
	var pr5, pr3 oligo.Oligo
	var total uint64

	flag.Parse()
	if *p5 != "" {
		var ok bool

		pr5, ok = long.FromString(*p5)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid 5'-end primer: %s\n", *p5)
			return
		}
	}

	if *p3 != "" {
		var ok bool

		pr3, ok = long.FromString(*p3)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid 3'-end primer: %s\n", *p3)
			return
		}
	}

	if *datasetFile == "" {
		fmt.Fprintf(os.Stderr, "Error: expecting synthesis file\n")
		return
	}

	dspool, err := utils.ReadPool([]string { *datasetFile }, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	var fnames []string
	for i := 0; i < flag.NArg(); i++ {
		fnames = append(fnames, flag.Arg(i))
	}

	fmt.Fprintf(os.Stderr, "Reading data\n")
	parse := fastq.Parse
	switch (*ftype) {
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid input file type: %s\n", *ftype)
	case "csv":
		parse = csv.Parse

	case "fastq":
	}

	pool, err := utils.ReadPool(fnames, true, parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if pr5 != nil && pr3 != nil {
		pool.Trim(pr5, pr3, *pdist, true)
		dspool.Trim(pr5, pr3, *pdist, true)
	}

	dspool.InitSearch()

	// find matches for each oligo in the dataset
	fmt.Fprintf(os.Stderr, "Matching %d reads to %d oligos...\n", pool.Size(), dspool.Size())
	ch := make(chan []*Match)
	nprocs := pool.Parallel(1024, func (ols []*utils.Oligo) {
		var ret []*Match
		for _, ol := range ols {
			ms := dspool.SearchMin(ol)

			if ms == nil {
				fmt.Fprintf(os.Stderr, "%d no match for %v\n", atomic.LoadUint64(&total), ol)
				continue
			}

			m := new(Match)
			m.oligo = ms.Seq
			m.seq = ol
			_, m.diff = oligo.Diff(m.oligo, m.seq)

			ret = append(ret, m)
			if atomic.AddUint64(&total, 1) % 1000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
			}
		}
	
		ch <- ret
	})

	// wait for the goroutines to finish, collect the matches found
	omap := make(map[oligo.Oligo][]*Match)
	for i := 0; i < nprocs; i++ {
		ms := <-ch
		for _, m := range ms {
			omap[m.oligo] = append(omap[m.oligo], m)
		}
	}

	// print oligos and the diffs
	for i, s := range dspool.Oligos() {
		ms := omap[s]
		if ms == nil {
			// no reads were mapped to this oligo
			if *printOligos {
				fmt.Printf("%d 0\n", i)
			} else {
				fmt.Printf("%d 0 %v\n", i, s)
			}
			continue
		}

		for _, m := range ms {
			if *printOligos {
				// print everything, takes more storage
				fmt.Printf("%d %d %v %v %v %v\n", i, m.seq.Count(), m.diff, m.seq.Qubundance(), m.oligo, m.seq)
			} else {
				// print only important stuff
				fmt.Printf("%d %d %v %v\n", i, m.seq.Count(), m.diff, m.seq.Qubundance())
			}

		}
	}
}
