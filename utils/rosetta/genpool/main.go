package main

import (
	"flag"
	"fmt"
	"math"
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
	newpos	int
}

type Ctx struct {
	pool	*Pool
	newp	*Pool
}

var olen = flag.Int("olen", 50, "oligo len")
var mindist = flag.Int("mindist", 15, "minimum distance")
var onum = flag.Int("onum", -1, "number of oligos")
var ds = flag.String("ds", "", "restart pool")
var seed = flag.Int("s", 0, "random generator seed")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var printds = flag.Bool("printds", true, "print the restart pool when you start")

var total uint64

func NewPool() *Pool {
	ret := new(Pool)
	ret.trie, _ = utils.NewTrie(nil)

	return ret
}

func (p *Pool) add(ol oligo.Oligo) {
	p.ols = append(p.ols, ol)
	p.trie = p.trie.AddClone(ol)
}

func (p *Pool) minDist(ol oligo.Oligo, mindist int) int {
	m := p.trie.SearchAtLeast(ol, mindist)
	if m == nil {
		return math.MaxInt
	}

	return m.Dist
}

func (p *Pool) clone() (ret *Pool) {
	ret = new(Pool)
	ret.ols = p.ols
	ret.newpos = len(ret.ols)
	ret.trie = p.trie

	return ret
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
	flag.Parse()

	count := 0
	inch := make(chan *Ctx)
	outch := make(chan *Ctx)
	pool := NewPool()

	rndseed := int64(*seed)
	if rndseed == 0 {
		rndseed = time.Now().Unix()
	}

	if *ds != "" {
		dspool, err := utils.ReadPool([]string{*ds}, false, csv.Parse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		for _, ol := range dspool.Oligos() {
			pool.add(ol)
			if *printds {
				fmt.Printf("%v %v 0 0\n", ol, count)
			}
			count++
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

	cts := make(map[*Ctx] *Pool)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			rnd := rand.New(rand.NewSource(int64(i) + (int64(rndseed) << 16)))

again:
			ctx := <-inch
			count := 0
			n := 0
			for {
				ol := genRand(rnd, *olen)
				if !criteria.H4G2.Check(ol) {
					continue
				}

				if gc := oligo.GCcontent(ol); gc < 0.4 || gc > 0.6 {
					continue
				}

				atomic.AddUint64(&total, 1)
				n++

				if ctx.pool.minDist(ol, *mindist) < *mindist {
					continue
				}

				if ctx.newp.minDist(ol, *mindist) < *mindist {
					continue
				}

				ctx.newp.add(ol)
				count++
				if (n > 1000 && count > 0) || count > 100 {
//					fmt.Fprintf(os.Stderr, ".")
					outch <- ctx
					goto again
				}
			}
		}()

		ctx := &Ctx { pool, NewPool() }
		cts[ctx] = NewPool()
		inch <- ctx
	}

	for *onum == -1 || count < *onum {
		select {
		case c := <- outch:
			var ols []oligo.Oligo

			t := atomic.LoadUint64(&total)
			p := cts[c]
			delete(cts, c)

			// collect all new oligos that are far enough
			for _, o := range c.newp.ols {
				mdist := p.minDist(o, *mindist)
				if mdist < *mindist {
					continue
				}

				ols = append(ols, o)
			}

			if ols != nil {
				pool = pool.clone()
				for _, o := range ols {
					pool.add(o)
					fmt.Printf("%v %v %v %v\n", o, count, *mindist, t)
					count++
				}

				// then add them to all contexts that are still valid
				for _, pp := range cts {
					for _, o := range ols {
						pp.add(o)
					}
				}
			}

			// send the goroutine a new context
			ctx := &Ctx { pool, NewPool() }
			cts[ctx] = NewPool()
			inch <- ctx

		case <- tch:
			return
		}
	}
}
