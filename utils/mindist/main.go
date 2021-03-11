package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"acoma/oligo"
_	"acoma/oligo/long"
	"acoma/io/csv"
	"acoma/utils"
//	"sort"
)

var synthFile = flag.String("s", "", "synthesis file")

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

	done := make(chan int)
	nprocs := dspool.Parallel(128, func (seqs []*utils.Oligo) {
		mindist := int(math.MaxUint32)
		for _, s1 := range seqs {
			for _, s2 := range dspool.Oligos() {
				if s1 == s2 {
					continue
				}

				d := oligo.Distance(s1, s2)
				if d < mindist {
					mindist = d
				}
			}
			fmt.Fprintf(os.Stderr, ".");
		}

		done <- mindist
	})

	mindist := int(math.MaxUint32)
	for i := 0; i < nprocs; i++ {
		d := <-done
		if d < mindist {
			mindist = d
		}
	}

	fmt.Printf("Minumum distance: %d\n", mindist)
}
