package main

import (
	"flag"
	"fmt"
_	"os"
	"sort"
_	"sync"
	"adscodex/oligo"
_	"adscodex/oligo/long"
_	"adscodex/utils/match/file"
)

type Eentry struct {
	prob	float64		// probability for the error to occur
	lendiff	int		// how does the entry change the length of the oligo
	ops	[]Eop		// actions
}

type Eop struct {
	op	byte		// D, I, or R
	nt	int		// the nt that was deleted, inserted, or replaced
	nt2	int		// for R, what the nt was replaced to
}

var ierrate = flag.Float64("ierr", 1.0, "insertion error rate (percent)")
var derrate = flag.Float64("derr", 1.0, "deletion error rate (percent)")
var serrate = flag.Float64("serr", 1.0, "substituion error rate (percent)")
var maxerrs = flag.Int("emdlmaxerrs", 10, "filter out entries with more than the specified number of errors")

func main() {
	flag.Parse()
	ents := generateErrorEntries(*ierrate/100, *derrate/100, *serrate/100, *maxerrs)

	for i := 0; i < len(ents); i++ {
		e := &ents[i]
		fmt.Printf("%v ", e.prob)
		for _, op := range e.ops {
			fmt.Printf("%c:%s", op.op, oligo.Nt2String(op.nt))
			if op.op == 'R' {
				fmt.Printf(":%s", oligo.Nt2String(op.nt2))
			}
		}
		fmt.Printf("\n")
	}
}

// given the insertion/deletion/substitution errors per position, generate the entries
func generateErrorEntries(ierr, derr, serr float64, maxerrs int) (ents []Eentry) {
	ents = genEentries(ents, 1, 0, nil, ierr, derr, serr, maxerrs)

	// calculate the total of the probabilities
	var total float64
	for i := 1; i < len(ents); i++ {
		total += ents[i].prob
	}

//	fmt.Printf("total %v\n", total)
	// update the default "no errors" with the remaining to 1
	ents[0].prob = 1 - total

	// sort
	sort.Slice(ents, func (i, j int) bool {
		return ents[i].prob > ents[j].prob
	})

//	for i := 0; i < len(ents); i++ {
//		fmt.Printf("%v\n", ents[i])
//	}
	return
}

func genEentries(ents []Eentry, prob float64, lendiff int, ops []Eop, ierr, derr, serr float64, maxerrs int) (ret []Eentry) {
	if maxerrs <= 0 {
		return ents
	}

	oplen := len(ops)
	ret = append(ents, Eentry { prob, lendiff, ops })

	// insert
	for nt := 0; nt < 4; nt++ {
		iops := make([]Eop, oplen + 1)
		copy(iops, ops)
		iops[oplen] = Eop { 'I', nt, -1 }
		ret = genEentries(ret, prob * (ierr / 4), lendiff - 1, iops, ierr, derr, serr, maxerrs - 1)
	}

	// delete
	for nt := 0; nt < 4; nt++ {
		dops := make([]Eop, oplen + 1)
		copy(dops, ops)
		dops[oplen] = Eop { 'D', nt, -1 }
		ret = genEentries(ret, prob * (derr / 4), lendiff + 1, dops, ierr, derr, serr, maxerrs - 1)
	}

	// substitution
	for nt := 0; nt < 4; nt++ {
		for nt2 := 0; nt2 < 4; nt2++ {
			sops := make([]Eop, oplen + 1)
			copy(sops, ops)
			sops[oplen] = Eop { 'R', nt, nt2 }
			ret = genEentries(ret, prob * (serr / 4), lendiff, sops, ierr, derr, serr, maxerrs - 1)
		}
	}

	return
}
