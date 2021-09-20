// Print number of reads (or cubundance) per original oligo.
// Uses match file from the match utility
package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"encoding/json"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/utils/match/file"
	"adscodex/utils/errmdl/moderate"
)

var maxerr = flag.Int("err", 0, "Maximum number of errors for match (0 - any)")
var p5primer = flag.String("p5", "", "5'-end primer")
var p3primer = flag.String("p3", "", "3'-end primer")
var dist = flag.Int("dist", 2, "number of errors allowed in the primers when matching")
var xprimers = flag.Bool("xp", false, "exclude primers from stats");
var gotable = flag.Bool("gotbl", false, "produce tables with Go syntax")
var pjson = flag.Bool("json", false, "print tables in JSON format")

func main() {
	var errins, errdel, errsub uint64		// number of insertion/deletion/substitution errors
	var ntcount uint64				// total number of nucleotides
	var insmap[4] uint64				// inserts per nt
	var delmap[4] uint64				// deletions per nt
	var submap[4][4] uint64				// substitutions per nt pair
	var ntmap[4] uint64				// total number of nts per nt
	var icount	map[int]uint64			// number of reads with n insertion errors (and no other errors)
	var dcount	map[int]uint64			// number of reads with n deletion errors (and no other errors)
	var scount	map[int]uint64			// number of reads with n substitution errors (and no other errors)
	var acount	map[int]uint64			// number of reads with n errors of any kind
	var ok bool
	var p3, p5 oligo.Oligo
	var p3len, p5len int
	var mutex sync.Mutex

	flag.Parse()
	if flag.NArg() < 1 {
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

	icount = make(map[int]uint64)
	dcount = make(map[int]uint64)
	scount = make(map[int]uint64)
	acount = make(map[int]uint64)
	n := 0
	var cnt uint64
	var err error
	for fn := 0; fn < flag.NArg(); fn++ {
		err = file.ParseParallel(flag.Arg(fn), 0, func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
			if *maxerr != 0 {
				nerr := 0
				for i := 0; i < len(diff); i++ {
					if diff[i] != '-' {
						nerr++
					}
				}

				if nerr > *maxerr {
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
//				fmt.Printf("%v\n%v\n%v\n", orig, read, diff)
			}

			var nts, erri, errd, errs int
			var imap, dmap, nmap [4]int
			var smap [4][4] int

			i, j := 0, 0
			olen := orig.Len()
			rlen := read.Len()
			nts += count * olen
			var icnt, dcnt, scnt int
			for a := 0; a < len(diff); a++ {
				if olen <= i || rlen <= j {
//					fmt.Printf("\nOrig: %v %d\n", orig, i)
//					fmt.Printf("Read: %v %d\n", read, j)
//					fmt.Printf("Diff: %v %d\n", diff, a)
				}
				switch diff[a] {
				case '-':
					// put the accurate matches in the table with the substitutions X->X
					c := orig.At(i)
					nmap[c] += count
					smap[c][c] += count
					i++
					j++

				case 'I':
					erri += count
					c := read.At(j)
					imap[c] += count
					icnt++
					j++

				case 'D':
					errd += count
					oc := orig.At(i)
					dmap[oc] += count
					dcnt++
					i++

				case 'R':
					errs += count
					oc := orig.At(i)
					c := read.At(j)
					nmap[oc] += count
					smap[oc][c] += count
					scnt++
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
				ntmap[i] += uint64(nmap[i])
			}

			if /*icnt != 0 &&*/ dcnt == 0 && scnt == 0 {
				icount[icnt] += uint64(count)
			}

			if /*dcnt != 0 &&*/ icnt == 0 && scnt == 0 {
				dcount[dcnt] += uint64(count)
			}

			if /*scnt != 0 &&*/ dcnt == 0 && icnt == 0 {
				scount[scnt] += uint64(count)
			}

			acount[icnt + dcnt + scnt] += uint64(count)
			n++
			cnt += uint64(count)
			mutex.Unlock()

			if n%100000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
			}
		})
		if err != nil {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	// test json
	if *pjson {
		var ed moderate.ModerateErrorDescr
		for i := 0; i < len(insmap); i++ {
			ed.Ins[i] = float64(insmap[i])/float64(ntcount)
		}
		for i := 0; i < len(delmap); i++ {
			ed.Del[i] = float64(delmap[i])/float64(ntcount)
		}
		for i := 0; i < len(submap); i++ {
			per := float64(ntmap[i])
			for j := 0; j < len(submap[i]); j++ {
				ed.Sub[i][j] = float64(submap[i][j])/per
			}
		}
		s, err := json.MarshalIndent(ed, "", "  ")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		fmt.Printf("%v", string(s))
		return
	}

	if !*gotable {
		fmt.Printf("Errors(%%):\n\tInsertion\t%v\n\tDeletion\t%v\n\tSubstitution\t%v\n\tTotal\t%v\n", 
			float64(errins*100)/float64(ntcount), float64(errdel*100)/float64(ntcount), float64(errsub*100)/float64(ntcount), float64((errins+errdel+errsub)*100)/float64(ntcount))
	}

	if !*gotable {
		fmt.Printf("Insertion errors per nt:(%%)\n")
		for i := 0; i < len(insmap); i++ {
			fmt.Printf("\t%v\t%.2f\t%v\n", oligo.Nt2String(i), float64(insmap[i] * 100)/float64(errins), float64(insmap[i] * 100)/float64(ntcount))
		}
	} else {
		fmt.Printf("var ins = []float64 { ")
		for i := 0; i < len(insmap); i++ {
			fmt.Printf("%v, ", float64(insmap[i])/float64(ntcount))
		}
		fmt.Printf("}\n\n")
	}

	if !*gotable {
		fmt.Printf("Deletion errors per nt:(%%)\n")
		for i := 0; i < len(delmap); i++ {
			fmt.Printf("\t%v\t%.2f\t%v\n", oligo.Nt2String(i), float64(delmap[i] * 100)/float64(errdel), float64(delmap[i] * 100)/float64(ntcount))
		}
	} else {
		fmt.Printf("var del = []float64 { ")
		for i := 0; i < len(delmap); i++ {
			fmt.Printf("%v, ", float64(delmap[i])/float64(ntcount))
		}
		fmt.Printf("}\n\n")
	}

	if !*gotable {
		per := float64(ntcount) / 100
		fmt.Printf("Substitution per nt:\n")

		fmt.Printf("\t")
		for i := 0; i < len(submap); i++ {
			fmt.Printf("\t%v", oligo.Nt2String(i))
		}
		fmt.Printf("\t*\n")
		for i := 0; i < len(submap); i++ {
			per := float64(ntmap[i]) / 100
			fmt.Printf("\t%v", oligo.Nt2String(i))
			var s uint64
			for j := 0; j < len(submap[i]); j++ {
				s += submap[i][j]
				fmt.Printf("\t%v", float64(submap[i][j])/per)
			}
			fmt.Printf("\t%v\n", float64(s)/per)
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
			fmt.Printf("\t%v", float64(ss)/per)
		}
		fmt.Printf("\t%v\n", float64(total)/per)
	} else {
		fmt.Printf("var sub = [][]float64 {\n")
		for i := 0; i < len(submap); i++ {
			per := float64(ntmap[i])
			fmt.Printf("\t{ ")
			for j := 0; j < len(submap[i]); j++ {
				fmt.Printf("%v, ", float64(submap[i][j])/per)
			}
			fmt.Printf(" },\n")
		}
		fmt.Printf("}\n\n")
	}

	if !*gotable {
		fmt.Printf("Errors per count\n")
		var tot float64
		for i := 0; i < 1000; i++ {
			c := acount[i]
			if c != 0 {
				f := float64(c * 100) / float64(cnt)
				tot += f
				fmt.Printf("\t%d\t%v\n", i, f)
//				fmt.Printf("\t%d\t%v\t%v\t%v\t%v\n", i, float64(c * 100) / float64(cnt), float64(icount[i]*100)/float64(cnt),float64(dcount[i]*100)/float64(cnt),float64(scount[i]*100)/float64(cnt))
			}
		}

		fmt.Printf("%v\n", tot)
	}
}
