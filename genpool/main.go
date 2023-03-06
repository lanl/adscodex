package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"strconv"
_	"sync"
	"sync/atomic"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
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
var crit = flag.String("c", "h4g2", "criteria")
var printds = flag.Bool("printds", true, "print the restart pool")
var pnum = flag.Int("p", 0, "number of procs")

var tstart, rtstart uint64	// starting values (if ds is specified)
var total uint64		// total that passed the criteria and the CG content
var ctotal uint64		// total that passed the criteria (but maybe not the CG content)
var rtotal uint64		// total that were randomly generated

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

	c := criteria.Find(*crit)
	if c == nil {
		fmt.Fprintf(os.Stderr, "Error: invalid criteria\n")
		return
	}

	if *ds != "" {
		ols, n, err := csvRead(*ds, *printds)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		for _, ol := range ols {
			pool.add(ol)
		}

		total = n
		count += len(ols)
		fmt.Fprintf(os.Stderr, "Loaded %d oligos\n", len(ols))
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
	procnum := runtime.NumCPU()
	if *pnum != 0 {
		procnum = *pnum
	}

	for i := 0; i < procnum; i++ {
		go func() {
			rnd := rand.New(rand.NewSource(int64(i) + (int64(rndseed) << 16)))

again:
			ctx := <-inch
			count := 0
			n := 0
			for {
				ol := genRand(rnd, *olen)
				if atomic.AddUint64(&rtotal, 1)%1000000 == 0 {
					fmt.Fprintf(os.Stderr, "-- %v %v %v\n", atomic.LoadUint64(&rtotal), atomic.LoadUint64(&ctotal), atomic.LoadUint64(&total))
				}
				if !c.Check(ol) {
					continue
				}

				atomic.AddUint64(&ctotal, 1)
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
			rt := atomic.LoadUint64(&rtotal)
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
					fmt.Printf("%v %v %v %v\n", o, count, t + tstart, rt + rtstart)
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

// Reads csv file produced by genpool
// Format is "seq count total ..."
func csvRead(fname string, print bool) (ols []oligo.Oligo, total uint64, err error) {
	var f *os.File
	var r io.Reader
	var n int

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	if cf, err := gzip.NewReader(f); err == nil {
		r = cf
	} else {
		r = f
		f.Seek(0, 0)
	}

	sc := bufio.NewScanner(r)
	n = 0
	for sc.Scan() {
		n++
		line := sc.Text()
		ls := strings.Split(line, " ")
		if len(ls) < 3 {
			err = fmt.Errorf("%d: invalid line: %s", n, line)
			return
		}

		seq := ls[0]
		tstart, err = strconv.ParseUint(ls[2], 10, 64)
		if err != nil {
			err = fmt.Errorf("%d: invalid count: %v: %v", n, ls[2], err)
			return
		}

		if len(ls) >= 4 {
			rtstart, err = strconv.ParseUint(ls[3], 10, 64)
			if err != nil {
				err = fmt.Errorf("%d: invalid count: %v: %v", n, ls[3], err)
				return
			}
		} else {
			rtstart = tstart
		}

		ol, ok := long.FromString(seq)
		if !ok {
			err = fmt.Errorf("%d: invalid sequence: %s", n, seq)
			return
		}

		if print {
			fmt.Printf("%s %v %v %v\n", seq, len(ols), tstart, rtstart)
		}

		ols = append(ols, ol)
	}

	return
}
