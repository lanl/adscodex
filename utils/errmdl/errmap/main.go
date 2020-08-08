package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/io/csv"
	"acoma/io/fastq"
	"acoma/utils"
)

var printOligos = flag.Bool("p", true, "print oligo and reads for each match")
var datasetFile = flag.String("ds", "", "synthesis file")
var pdist = flag.Int("pdist", 3, "primer match distance")
var mdist = flag.Int("maxdist", 64, "maximum distance for matching")
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")

type Match struct {
	oligo	oligo.Oligo		// the original oligo from the synthesis file
	seq	*utils.Oligo		// the sequence from the results
	diff	string			// difference from the oligo to seq
	count	int			// number of reads of the sequence
}

func main() {
	var pr5, pr3 oligo.Oligo

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
	pool, err := utils.ReadPool(fnames, true, fastq.Parse)
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
	ch := make(chan []*Match)
	nprocs := pool.Parallel(1024, func (ols []*utils.Oligo) {
		var ret []*Match
		for _, ol := range ols {
			var mss []utils.DistSeq
			var d int

			// find some matches
			for d = 1; d <= *mdist && (mss==nil || len(mss) == 0); {
				mss = dspool.Search(ol, d)
				d *= 2
			}

			// find the closest distance
			for _, ms := range mss {
				if ms.Dist < d {
					d = ms.Dist
				}
			}

			// find the closest matches
			var matches []oligo.Oligo
			for _, ms := range mss {
				if ms.Dist == d {
					matches = append(matches, ms.Seq)
				}
			}

			var match oligo.Oligo
			switch len(matches) {
			case 0:
				fmt.Fprintf(os.Stderr, "no match for %v\n", ol)
				continue

			case 1:
				match = matches[0]

			default:
				// if there are multiple matches, choose one randomly
				fmt.Fprintf(os.Stderr, "multiple matches for %v: dist %d matches %d\n", ol, d, len(matches))
				match = matches[rand.Intn(len(matches))]
			}

			m := new(Match)
			m.oligo = match
			m.seq = ol
			_, m.diff = oligo.Diff(m.oligo, m.seq)

			ret = append(ret, m)
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
				fmt.Printf("%d %d %v %v %v %v\n", i, m.count, m.diff, m.seq.Qubundance(), m.count, m.oligo, m.seq)
			} else {
				// print only important stuff
				fmt.Printf("%d %d %v %v\n", i, m.count, m.diff, m.seq.Qubundance())
			}

		}
	}
}
