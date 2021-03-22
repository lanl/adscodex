// Print number of reads (or cubundance) per original oligo.
// Uses match file from the match utility
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"adscodex/oligo"
	"adscodex/utils/match/file"
)

type entry struct {
	count int
	cubu float64
}

var scrit = flag.String("sort", "", "Sorting criteria ('', 'count' or 'cubundance')")
var err = flag.Int("err", 0, "Maximum number of errors for match (0 - any)")

func main() {
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "expecting match file name\n")
		return
	}

	ms := make(map[int]*entry)
	n := 0
	err := file.Parse(flag.Arg(0), func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
		if *err != 0 {
			nerr := 0
			for i := 0; i < len(diff); i++ {
				if diff[i] != '-' {
					nerr++
				}
			}

			if *err < nerr {
				return
			}
		}

		if m, ok := ms[id]; ok {
			m.count += count
			m.cubu += cubu
		} else {
			m := new(entry)
			m.count = count
			m.cubu = cubu
			ms[id] = m
		}

		n++
		if n%100000 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%d entries\n", len(ms))
	entries := make([]entry, len(ms))
	for i, m := range ms {
		entries[i] = *m
	}

	var less func(i, j int) bool
	switch *scrit {
	case "count":
		less = func(i, j int) bool {
			return entries[i].count > entries[j].count
		}

	case "cubundance":
		less = func(i, j int) bool {
			return entries[i].cubu > entries[j].cubu
		}
	}

	if less != nil {
		sort.Slice(entries, less)
	}

	for i := 0; i < len(entries); i++ {
		fmt.Printf("%d %d %v\n", i, entries[i].count, entries[i].cubu)
	}
}
