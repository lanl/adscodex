// Print number of reads (or cubundance) per original oligo.
// Uses match file from the match utility
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
	"adscodex/oligo"
	"adscodex/utils/match/file"
	"adscodex/utils/errmdl/newer"
)

type Rec struct {
	count	int
	olen	int
	imap	[4]int
	dmap	[4]int
	nmap	[4]int
	smap	[4][4]int
}

var maxerr = flag.Int("maxerr", 0, "Maximum number of errors for match (0 - any)")
var dictnum = flag.Int("dictnum", 0, "Number of dictionary oligos (if 0 uses the maximum id found in the file(s)")

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "expecting match file name\n")
		return
	}

	recs, dnum, dmax, err := parseRecords()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	maxdnum := *dictnum
	if maxdnum == 0 {
		maxdnum = dmax
	}

	// calculate means and distribution
	var icnt, dcnt, scnt, cnt uint64
	var omax, onum, dolen int
	cimap := make(map[int]uint64)	// histogram taking acount into count
	imap := make(map[int]uint64)	// histogram counting each unique oligo as one
	cdmap := make(map[int]uint64)	// histogram taking acount into count
	dmap := make(map[int]uint64)	// histogram counting each unique oligo as one
	csmap := make(map[int]uint64)	// histogram taking acount into count
	smap := make(map[int]uint64)	// histogram counting each unique oligo as one
	for _, r := range recs {
		dolen = r.olen
		e := 0
		for _, n := range r.imap {
			icnt += uint64(n*r.count)
			e += n
		}
		cimap[e] += uint64(r.count)
		imap[e]++

		e = 0
		for _, n := range r.dmap {
			dcnt += uint64(n*r.count)
			e += n
		}
		cdmap[e] += uint64(r.count)
		dmap[e]++

		e = 0
		for i := 0; i < len(r.smap); i++ {
			for j, n := range r.smap[i] {
				if i == j {
					continue
				}

				scnt += uint64(n*r.count)
				e += n
			}
		}
		csmap[e] += uint64(r.count)
		smap[e]++

		cnt += uint64(r.olen * r.count)
		if omax < r.olen {
			omax = r.olen
		}

		onum += r.count
	}

	imean := float64(icnt)/float64(cnt)
	dmean := float64(dcnt)/float64(cnt)
	smean := float64(scnt)/float64(cnt)

	// calculate variance
	var ivar, dvar, svar float64
	for _, r := range recs {
		var icnt, dcnt, scnt int

		for _, n := range r.imap {
			icnt += n
		}

		for _, n := range r.dmap {
			dcnt += n
		}

		for i := 0; i < len(r.smap); i++ {
			for _, n := range r.smap[i] {
				scnt += n
			}
		}

		cnt := float64(r.count)
		v := float64(icnt) - imean
		ivar += cnt * v * v

		v = float64(dcnt) - dmean
		dvar += cnt * v * v

		v = float64(scnt) - smean
		svar += cnt * v * v
	}

	var nd newer.NewErrorDescr
	nd.Dropout = float64(maxdnum - dnum)/float64(maxdnum)
	nd.OLen = dolen
	nd.InsErr = imean
	nd.SubErr = smean
	nd.DelErr = dmean

	// insertion error distribution
	n := 0
	for i, _ := range cimap {
		if i > n {
			n = i
		}
	}
	nd.InsDist = make([]float64, n + 1)
	for i, v := range cimap {
		nd.InsDist[i] = float64(v)/float64(onum)
	}

	// substitution error distribution
	n = 0
	for i, _ := range csmap {
		if i > n {
			n = i
		}
	}
	nd.SubDist = make([]float64, n + 1)
	for i, v := range csmap {
		nd.SubDist[i] = float64(v)/float64(onum)
	}

	// deletion error distribution
	n = 0
	for i, _ := range cdmap {
		if i > n {
			n = i
		}
	}
	nd.DelDist = make([]float64, n + 1)
	for i, v := range cdmap {
		nd.DelDist[i] = float64(v)/float64(onum)
	}

	s, err := json.MarshalIndent(nd, "", "  ")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Printf("%v", string(s))

/*
	fmt.Printf("Dictionary matches: %d\n", dnum)
	fmt.Printf("Insertion error: %v %v\n", imean, ivar)
	fmt.Printf("Deletion error: %v %v\n", dmean, dvar)
	fmt.Printf("Substitution error: %v %v\n", smean, svar)

	fmt.Printf("Number of oligos per number of errors:\n")
	for i := 0; i < omax; i++ {
		civ := float64(cimap[i])/float64(onum)
		iv :=  float64(imap[i])/float64(len(recs))
		cdv := float64(cdmap[i])/float64(onum)
		dv :=  float64(dmap[i])/float64(len(recs))
		csv := float64(csmap[i])/float64(onum)
		sv :=  float64(smap[i])/float64(len(recs))
		if civ==0 && iv==0 && cdv==0 && dv==0 && csv==0 && sv==0 {
			continue
		}

		fmt.Printf("\t%d\t%v\t%v\t%v\t%v\t%v\t%v\n", i, civ, iv, cdv, dv, csv, sv)
	}
*/
}

func parseRecords() (recs []*Rec, dictnum, dictmax int, err error) {
	var mutex sync.Mutex
	var n int

	dmatch := make(map[int]int)
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

//			var nts, erri, errd, errs int
//			var imap, dmap, nmap [4]int
//			var smap [4][4] int

			r := new(Rec)
			r.olen = orig.Len()
			r.count = count
			i, j := 0, 0
			for a := 0; a < len(diff); a++ {
				switch diff[a] {
				case '-':
					// put the accurate matches in the table with the substitutions X->X
					c := orig.At(i)
					r.nmap[c]++
					r.smap[c][c]++
					i++
					j++

				case 'I':
					c := read.At(j)
					r.imap[c]++
					j++

				case 'D':
					oc := orig.At(i)
					r.dmap[oc]++
					i++

				case 'R':
					oc := orig.At(i)
					c := read.At(j)
					r.nmap[oc]++
					r.smap[oc][c]++
					i++
					j++
				}
			}

			mutex.Lock()
			dmatch[id]++
			if id > dictmax {
				dictmax = id
			}

			recs = append(recs, r)
			n++
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

	dictnum = len(dmatch)
	return
}
