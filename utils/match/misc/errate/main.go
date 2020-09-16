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

func main() {
	var erri, errd, errs uint64	// number of insertion/deletion/substitution errors
	var nts uint64			// total number of nucleotides
	var ok bool
	var p3, p5 oligo.Oligo
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
	}

	if *p3primer != "" {
		p3, ok = long.FromString(*p3primer)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid 5'-end primer: %v\n", *p5primer)
		}
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
			ppos, _ := oligo.Find(read, p5, *dist)
			if ppos == -1 {
				return
			}

			spos, slen := oligo.Find(read, p3, *dist)
			if spos == -1 {
				return
			}
//			fmt.Printf("ppos %d spos %d slen %d\n", ppos, spos, slen)
			send := spos + slen

			read = read.Slice(ppos, send)
			_, diff = oligo.Diff(orig, read)
//			fmt.Printf("%v\n%v\n%v\n", orig, read, diff)
		}

		c64 := uint64(count)
		mutex.Lock()
		for i := 0; i < len(diff); i++ {
			nts += c64
			switch diff[i] {
			case 'I':
				erri += c64

			case 'D':
				errd += c64

			case 'R':
				errs += c64
			}
		}
		mutex.Unlock()

		n++
		if n%100000 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}
	})

	fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Printf("Errors(%%): insertion %v deletion %v substitution %v\n", float64(erri*100)/float64(nts), float64(errd*100)/float64(nts), float64(errs*100)/float64(nts))
}
