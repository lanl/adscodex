package main

import (
	"flag"
	"fmt"
_	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
_	"sync"
	"sync/atomic"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/io/csv"
	"adscodex/utils"
	"adscodex/criteria"
)

type Pool struct {
	ols	[]oligo.Oligo
	trie	*utils.Trie
	pfx	[]*utils.Oligo
	newpos	int
}

var olen = flag.Int("olen", 50, "oligo len")
var mindist = flag.Int("mindist", 15, "minimum distance")
var onum = flag.Int("onum", -1, "number of oligos")
var ds = flag.String("ds", "", "restart pool")
var seed = flag.Int("s", 0, "random generator seed")
var pfxfile = flag.String("pfx", "", "prefix file")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

var total uint64

func NewPool(pfx []*utils.Oligo) *Pool {
	ret := new(Pool)
	ret.pfx = pfx
	ret.trie, _ = utils.NewTrie(nil)

	return ret
}

func (p *Pool) add(ol oligo.Oligo) {
	p.ols = append(p.ols, ol)
	if p.pfx == nil {
		if err := p.trie.Add(ol, 0); err != nil {
			panic(err)
		}
		return
	}

	for _, pol := range p.pfx {
		lol := pol.Clone()
		lol.Append(ol)
		if err := p.trie.Add(lol, 0); err != nil {
			panic(err)
		}
	}
}

func (p *Pool) checkDist(ol oligo.Oligo, mindist int) (ok bool) {
	ok = true
	if p.pfx == nil {
		m := p.trie.SearchAtLeast(ol, mindist)
		if m != nil {
			ok = m.Dist > mindist
		}

		return
	}

	for _, pol := range p.pfx {
		lol := pol.Clone()
		lol.Append(ol)

		m := p.trie.SearchAtLeast(lol, mindist)
		if m != nil {
			ok = m.Dist > mindist
			if !ok {
				break
			}
		}
	}

	return
}

func (p *Pool) clone() (ret *Pool) {
	ret = new(Pool)
	ret.pfx = p.pfx
	if p.ols != nil {
		ret.ols = make([]oligo.Oligo, len(p.ols))
		copy(ret.ols, p.ols)
		ret.newpos = len(ret.ols)
	}

	ret.trie = p.trie.Clone()

	return ret
}

func (p *Pool) check(ol oligo.Oligo) (ok bool) {
	if p.pfx == nil {
		return criteria.H4G2.Check(ol)
	}

	ok = true
	for _, pol := range p.pfx {
		lol := pol.Clone()
		lol.Append(ol)
		if !criteria.H4G2.Check(lol) {
			ok = false
			break
		}
	}

	return ok
}
func genRand(r *rand.Rand, olen int) (ret oligo.Oligo) {
	ret = long.New(olen)
	for n := 0; n < olen; {
		v := r.Uint64()
		for i := 0; i < 16 && n < olen; i++ {
			ret.Set(n, int(v&3))
			v >>= 2
			n++
		}
	}

	return
}

func main() {
	var pfx []*utils.Oligo

	flag.Parse()

	inch := make(chan *Pool)
	outch := make(chan *Pool)

	if *pfxfile != "" {
		pfxpool, err := utils.ReadPool([]string{*pfxfile}, false, csv.Parse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		pfx = pfxpool.Oligos()
	}

	pool := NewPool(pfx)
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

	var tch <-chan time.Time
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n")
			return
		}

		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		tch = time.Tick(5 * time.Minute)
	}

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			rnd := rand.New(rand.NewSource(int64(i) + (int64(*seed) << 16)))

again:
			pool := <-inch
			p := pool.clone()
			count := 0
			n := 0
			for {
				ol := genRand(rnd, *olen)
				if !p.check(ol) {
					continue
				}

				atomic.AddUint64(&total, 1)
				n++

				if p.checkDist(ol, *mindist) {
					p.add(ol)
					count++
				}

				if (n > 100000 && count > 0) || count > 10000 {
					outch <- p
					goto again
				}
			}
		}()

		inch <- pool
	}

	count := 0
	for *onum == -1 || count < *onum {
		select {
		case p := <- outch:
			for i := p.newpos; i < len(p.ols) && (*onum ==-1 || count < *onum); i++ {
				o := p.ols[i]
				if !pool.checkDist(o, *mindist) {
					continue
				}

				fmt.Printf("%v %d %d\n", o, count, atomic.LoadUint64(&total))
				pool.add(o)
				count++
			}

			inch <- pool
		case <- tch:
			return
		}
	}
}
