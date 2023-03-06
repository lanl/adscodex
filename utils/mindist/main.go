package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"adscodex/oligo"
_	"adscodex/oligo/long"
	"adscodex/io/csv"
	"adscodex/utils"
//	"sort"
)

var synthFile = flag.String("s", "", "synthesis file")

type Stat struct {
	ol	oligo.Oligo
	mindist	int
	mdols	[]oligo.Oligo
	avgdist	float64
}

func main() {
	var files []string

	flag.Parse()
	for i := 0; i < flag.NArg(); i++ {
		files = append(files, flag.Arg(i))
	}

	dspool, err := utils.ReadPool(files, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	ols := dspool.Oligos()
	done := make(chan []Stat)

	var total uint32
	pcent := uint32(len(ols)/100)
	nprocs := dspool.Parallel(256, func (seqs []*utils.Oligo) {
		var stats []Stat

		for _, s1 := range seqs {
			var avgdist float64
			mindist := int(math.MaxInt32)
			var mdols []oligo.Oligo
			for _, s2 := range ols {
				if s1 == s2 {
					continue
				}

				d := oligo.Distance(s1, s2)
				if d < mindist {
					mindist = d
					mdols = append([]oligo.Oligo(nil), s2)
				} else if d == mindist {
					mdols = append(mdols, s2)
				}

				avgdist += float64(d)
			}

			t := atomic.AddUint32(&total, 1)
			if t%pcent == 0 {
				fmt.Fprintf(os. Stderr, ".")
			}

			avgdist /= float64(len(ols) - 1)
			stats = append(stats, Stat { s1, mindist, mdols, avgdist })
		}

		done <- stats
	})

	var stats []Stat

	minmdist := int(math.MaxInt32)
	maxmdist := 0
	var avgmdist, avgdist float64
	for i := 0; i < nprocs; i++ {
		sts := <-done
		stats = append(stats, sts...)
		for i := 0; i < len(sts); i++ {
			st := &sts[i]
			if st.mindist < minmdist {
				minmdist = st.mindist
			}

			if st.mindist > maxmdist {
				maxmdist = st.mindist
			}

			avgmdist += float64(st.mindist)
			avgdist += st.avgdist
		}
	}

	avgmdist /= float64(len(ols))
	avgdist /= float64(len(ols))

	fmt.Printf("\nMinumum min distance: %d\n", minmdist)
	fmt.Printf("Maximum min distance: %d\n", maxmdist)
	fmt.Printf("Average min distance: %v\n", avgmdist)
	fmt.Printf("Average distance: %v\n", avgdist)
}
