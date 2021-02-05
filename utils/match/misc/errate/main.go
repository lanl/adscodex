// Print number of reads (or cubundance) per original oligo.
// Uses match file from the match utility
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/utils/match/file"
)

var err = flag.Int("err", 0, "Maximum number of errors for match (0 - any)")
var p5primer = flag.String("p5", "", "5'-end primer")
var p3primer = flag.String("p3", "", "3'-end primer")
var dist = flag.Int("dist", 2, "number of errors allowed in the primers when matching")
var xprimers = flag.Bool("xp", false, "exclude primers from stats");

func main() {
	var errins, errdel, errsub uint64		// number of insertion/deletion/substitution errors
	var ntcount uint64				// total number of nucleotides
	var insmap[4] uint64
	var delmap[4] uint64
	var submap[4][4]	uint64	// count per nt
	var ok bool
	var p3, p5 oligo.Oligo
	var p3len, p5len int
	var mutex sync.Mutex

	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "expecting match file name\n")
		return
	}

	if *p5primer != "" {
		p5, ok = long.FromString(*p5primer)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid 5'-end primer: %v\n", *p5primer)
		}
		p5len = p5.Len()
	}

	if *p3primer != "" {
		p3, ok = long.FromString(*p3primer)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid 5'-end primer: %v\n", *p5primer)
		}
		p3len = p3.Len()
	}

	n := 0
	err := file.ParseParallel(flag.Arg(0), 0, func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
		if *err != 0 {
			nerr := 0
			for i := 0; i < len(diff); i++ {
				if diff[i] != '-' {
					nerr++
				}
			}

			if nerr > *err {
				return
			}
		}

		if p5 != nil && p3 != nil {
			// ignore reads that don't have the primers
			ppos, plen := oligo.Find(read, p5, *dist)
			if ppos == -1 {
				return
			}

			spos, slen := oligo.Find(read, p3, *dist)
			if spos == -1 {
				return
			}

			if (*xprimers) {
				ppos += plen
				orig = orig.Slice(p5len, orig.Len() - p3len)
			} else {
				spos += slen
			}

			read = read.Slice(ppos, spos)
			_, diff = oligo.Diff(orig, read)
//			fmt.Printf("%v\n%v\n%v\n", orig, read, diff)
		}

		var nts, erri, errd, errs int
		var imap, dmap [4]int
		var smap [4][4] int

		i, j := 0, 0
		olen := orig.Len()
		rlen := read.Len()
		nts += count * olen
		for a := 0; a < len(diff); a++ {
			if olen <= i || rlen <= j {
//				fmt.Printf("\nOrig: %v %d\n", orig, i)
//				fmt.Printf("Read: %v %d\n", read, j)
//				fmt.Printf("Diff: %v %d\n", diff, a)
			}
			switch diff[a] {
			case '-':
				i++
				j++

			case 'I':
				erri += count
				c := read.At(j)
				imap[c] += count
				j++

			case 'D':
				errd += count
				oc := orig.At(i)
				dmap[oc] += count
				i++

			case 'R':
				errs += count
				oc := orig.At(i)
				c := read.At(j)
				smap[oc][c] += count
				i++
				j++
			}
		}

		mutex.Lock()
		ntcount += uint64(nts)
		errins += uint64(erri)
		errdel += uint64(errd)
		errsub += uint64(errs)
		for i := 0; i < len(imap); i++ {
			insmap[i] += uint64(imap[i])
			delmap[i] += uint64(dmap[i])
			for j := 0; j < len(smap[i]); j++ {
				submap[i][j] += uint64(smap[i][j])
			}
		}
		n++
		mutex.Unlock()

		if n%100000 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}
	})

	fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Printf("Errors(%%):\n\tInsertion\t%v\n\tDeletion\t%v\n\tSubstitution\t%v\n\tTotal\t%v\n", 
		float64(errins*100)/float64(ntcount), float64(errdel*100)/float64(ntcount), float64(errsub*100)/float64(ntcount), float64((errins+errdel+errsub)*100)/float64(ntcount))

	fmt.Printf("Insertion per nt:\n")
	for i := 0; i < len(insmap); i++ {
		fmt.Printf("\t%v\t%.2f\t%v\n", oligo.Nt2String(i), float64(insmap[i] * 100)/float64(errins), float64(insmap[i] * 100)/float64(ntcount))
	}

	fmt.Printf("Deletion per nt:\n")
	for i := 0; i < len(delmap); i++ {
		fmt.Printf("\t%v\t%.2f\t%v\n", oligo.Nt2String(i), float64(delmap[i] * 100)/float64(errdel), float64(delmap[i] * 100)/float64(ntcount))
	}

	fmt.Printf("Substitution per nt:\n")
	fmt.Printf("\t")
	for i := 0; i < len(submap); i++ {
		fmt.Printf("\t%v", oligo.Nt2String(i))
	}
	fmt.Printf("\t*\n")
	for i := 0; i < len(submap); i++ {
		fmt.Printf("\t%v", oligo.Nt2String(i))
		var s uint64
		for j := 0; j < len(submap[i]); j++ {
			s += submap[i][j]
			fmt.Printf("\t%.2f", float64(submap[i][j] * 100)/float64(errsub))
		}
		fmt.Printf("\t%.2f\n", float64(s * 100)/float64(errsub))
	}

	var s[4] uint64
	for i := 0; i < len(submap); i++ {
		for j := 0; j < len(submap[i]); j++ {
			s[j] += submap[i][j]
		}
	}

	var total uint64
	fmt.Printf("\t*")
	for _, ss := range s {
		total += ss
		fmt.Printf("\t%.2f", float64(ss * 100)/float64(errsub))
	}
	fmt.Printf("\t%.2f\n", float64(total * 100)/float64(errsub))
}
