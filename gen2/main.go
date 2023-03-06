package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
_	"runtime"
_	"runtime/pprof"
_	"sync"
_	"sync/atomic"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/io/csv"
	"adscodex/utils"
_	"adscodex/criteria"
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
	pool := NewPool()

	rndseed := int64(*seed)
	if rndseed == 0 {
		rndseed = time.Now().Unix()
	}

	dspool, err := utils.ReadPool([]string{flag.Arg(0)}, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	for i, ol := range dspool.Oligos() {
		if pool.minDist(ol, *mindist) < *mindist {
			continue
		}

		pool.add(ol)
		fmt.Printf("%v %v %v\n", ol, count, i)
		count++
	}

	fmt.Fprintf(os.Stderr, "Loaded %d oligos\n", len(dspool.Oligos()))
}
