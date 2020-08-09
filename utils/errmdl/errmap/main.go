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
	dist	int			// distance
	diff	string			// difference from the oligo to seq
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

//	dspool.InitSearch()

	// find matches for each oligo in the dataset
	ch := make(chan []*Match)
	omap := make(map[oligo.Oligo] []*Match)
	for dist := 1; dist <= 64; dist *= 2 {
		fmt.Fprintf(os.Stderr, "Distance %d\n", dist)
		fmt.Fprintf(os.Stderr, "\tBuilding trie...")
		pool.InitSearch()
		fmt.Fprintf(os.Stderr, "\n")

		nprocs := dspool.Parallel(1024, func (ols []*utils.Oligo) {
			var ret []*Match
			for _, ol := range ols {
				mss := pool.Search(ol, dist)
				for _, ms := range mss {
					m := new(Match)
					m.oligo = ol
					m.seq = ms.Seq.(*utils.Oligo)
					m.dist = ms.Dist
//					_, m.diff = oligo.Diff(m.oligo, m.seq)
					ret = append(ret, m)
				}
			}

			ch <- ret
		})

		// collect all the matches and process them
		matches := make(map[*utils.Oligo] []*Match)
		fmt.Fprintf(os.Stderr, "\tCollecting matches ")
		for i := 0; i < nprocs; i++ {
			ms := <- ch
			fmt.Fprintf(os.Stderr, ".")
			for _, m := range ms {
				if ma, found := matches[m.seq]; found {
					if ma[0].dist > m.dist {
						// found a shorter distance
						ma = nil
					} else if ma[0].dist == m.dist {
						// same distance, append
					}

					matches[m.seq] = append(ma, m)
				} else {
					matches[m.seq] = append([]*Match(nil), m)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "\n")

		// for each matched read pick a random match (if more than one)
		// and remove from the pool
		var fseqs []*utils.Oligo
		for _, ms := range matches {
			var m *Match

			if len(ms) == 1 {
				m = ms[0]
			} else {
				// if there are multiple matches, choose one randomly
				m = ms[rand.Intn(len(ms))]
				fmt.Fprintf(os.Stderr, "multiple matches for %v: dist %d matches %d\n", m.seq, m.dist, len(ms))
			}

			omap[m.oligo] = append(omap[m.oligo], m)
			fseqs = append(fseqs, m.seq)
		}

		pool.Remove(fseqs)
		fmt.Fprintf(os.Stderr, "\tMatched %d sequences, remaining %d\n", len(fseqs), pool.Size())
		if pool.Size() == 0 {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "Calculating diffs")
	nprocs := dspool.Parallel(1024, func (ols []*utils.Oligo) {
		for _, o := range ols {
			ms := omap[o]
			for _, m := range ms {
				_, m.diff = oligo.Diff(m.oligo, m.seq)
			}
		}
		ch <- nil
	})

	for i := 0; i < nprocs; i++ {
		<- ch
	}
	fmt.Fprintf(os.Stderr, "\n")

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
