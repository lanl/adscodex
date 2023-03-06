package main

import (
	"flag"
	"fmt"
_	"math"
_	"math/rand"
	"os"
_	"runtime"
	"runtime/pprof"
_	"sync"
_	"sync/atomic"
_	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/oligo/short"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/criteria"
)

type Pool struct {
	ols	[]oligo.Oligo
	trie	*utils.Trie
	newpos	int
}

var olen = flag.Int("olen", 50, "oligo len")
var mindist = flag.Int("mindist", 15, "minimum distance")
var onum = flag.Int("onum", -1, "number of oligos")
var ds = flag.String("ds", "", "restart pool")
var seed = flag.Int("s", 0, "random generator seed")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var prefix = flag.String("prefix", "", "oligo prefix")
var startol = flag.String("start", "", "starting oligo")

var total uint64

func NewPool() *Pool {
	ret := new(Pool)
	ret.trie, _ = utils.NewTrie(nil)

	return ret
}

func (p *Pool) add(ol oligo.Oligo) {
	p.ols = append(p.ols, ol)
	if err := p.trie.Add(ol, 0); err != nil {
		panic(err)
	}
}

func (p *Pool) minDist(ol oligo.Oligo, mindist int) int {
	m := p.trie.SearchMin(ol)
	if m != nil {
		return m.Dist
	}

	return *olen
}

func (p *Pool) clone() (ret *Pool) {
	ret = new(Pool)
	if p.ols != nil {
		ret.ols = make([]oligo.Oligo, len(p.ols))
		copy(ret.ols, p.ols)
		ret.newpos = len(ret.ols)
	}

	ret.trie = p.trie.Clone()

	return ret
}

func main() {
	var pfx *long.Oligo
	var ok bool

	flag.Parse()

	pool := NewPool()
	if *ds != "" {
		dspool, err := utils.ReadPool([]string{*ds}, false, csv.Parse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		for _, ol := range dspool.Oligos() {
			pool.add(ol)
		}

		fmt.Fprintf(os.Stderr, "Loaded %d oligos\n", len(dspool.Oligos()))
	}

	if *prefix != "" {
		pfx, ok = long.FromString(*prefix)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid prefix %v\n", *prefix)
		}
	}

//	var tch <-chan time.Time
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n")
			return
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
//		tch = time.Tick(5 * time.Minute)
	}

	var total float64
	total = float64(uint64(1)<<(2* *olen))

	stol := short.New(*olen)
	if *startol != "" {
		stol, ok = short.FromString(*startol)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid starting oligo: %v\n", *startol)
			return
		}

		if stol.Len() != *olen {
			fmt.Fprintf(os.Stderr, "Error: starting oligo is not %d nts\n", *olen)
			return
		}
	}

	ol := stol
	n := 0
	for {
		var nol oligo.Oligo
		if pfx != nil {
			nol = pfx.Clone()
			nol.Append(ol)
		} else {
			nol = ol
		}

		if gc := oligo.GCcontent(ol); (gc < 0.4 || gc > 0.6) && criteria.H4.Check(nol) && pool.minDist(ol, *mindist) >= *mindist {
			pool.add(nol)
			fmt.Printf("%v %d %4.2f\n", ol, n, (float64(ol.Uint64())*100)/total)
			n++
		}

		if !ol.Next() {
			ol = short.New(*olen)
			if ol.Cmp(stol) == 0 {
				break
			}
		}
	}
}
