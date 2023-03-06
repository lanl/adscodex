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

var mindist = flag.Int("mindist", 15, "minimum distance")
var seed = flag.Int("s", 0, "random generator seed")
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

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
	var idx uint64

	flag.Parse()

	count := 0
	inch := make(chan *Ctx)
	outch := make(chan *Ctx)
	pool := NewPool()

	rndseed := int64(*seed)
	if rndseed == 0 {
		rndseed = time.Now().Unix()
	}

	pool1, err := utils.ReadPool([]string{flag.Arg(0)}, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	ols1 := pool1.Oligos()
	n1 := len(ols1)
	fmt.Fprintf(os.Stderr, "Pool1 %d oligos\n", n1)

	pool2, err := utils.ReadPool([]string{flag.Arg(1)}, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	ols2 := pool2.Oligos()
	n2 := len(ols2)
	fmt.Fprintf(os.Stderr, "Pool2 %d oligos\n", n2)

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
	procnum := runtime.NumCPU()
	for i := 0; i < procnum; i++ {
		go func() {
			var i, j int

			ctx := <-inch
			count := 0
			n := 0
			for i < n1 {
				cidx := atomic.AddUint64(&idx, 1) - 1
				i = int(cidx / uint64(n2))
				j = int(cidx % uint64(n2))

				if i >= n1 {
					continue
				}

				ol := ols1[i].Clone()
				ol.Append(ols2[j])
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
					ctx = <-inch
					count = 0
					n = 0
				}
			}

			// send the last oligos and signal the end
			outch <- ctx
			<-inch
			outch <- nil
		}()

		ctx := &Ctx { pool, NewPool() }
		cts[ctx] = NewPool()
		inch <- ctx
	}

	for procnum > 0 {
		select {
		case c := <- outch:
			var ols []oligo.Oligo
			if c == nil {
				procnum--
				continue
			}

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
					count++
					pool.add(o)
					fmt.Printf("%v %v %v\n", o, count, t)
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
