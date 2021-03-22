package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"time"
	"container/heap"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
	"adscodex/io/fastq"
	"adscodex/io/csv"
	"adscodex/criteria"
	"adscodex/utils"
)

type Kmer string
type Kpair struct {
	k	Kmer
	n	float64
}

type Graph struct {
	sync.Mutex
	klen	int		// kmer length

	kmap	map[Kmer]int
	heap	[]Kpair
	heapinit bool
}

type Oligo struct {
	ol	oligo.Oligo
	lkmer	Kmer
	rkmer	Kmer
	mval	float64
	kmap	map[Kmer] bool
}

type Seq struct {
	seq	string
	quality	[]byte
	rev	bool
}

var ftype = flag.String("t", "fastq", "file type")
var klen = flag.Int("k", 31, "kmer length")
var profname = flag.String("prof", "", "profile filename")
var noligos = flag.Int("n", 0, "number of oligos")
var maxlen = flag.Int("l", 147, "maximum oligo length")
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")
var minqual = flag.Float64("qk", 0.1, "minimum quality of the initial kmer")
var useqscore = flag.Bool("q", true, "use quality scores (if available)")

func NewGraph(kmerLen int) *Graph {
	g := new(Graph)
	g.kmap = make(map[Kmer]int)
	g.klen = kmerLen
	g.heap = nil

	return g
}

func (g *Graph) Add(k Kmer, q float64) {
	g.Lock()
	if n, found := g.kmap[k]; found {
		g.heap[n].n += q
	} else {
		g.kmap[k] = len(g.heap)
		g.heap = append(g.heap, Kpair { k, q })
	}
	g.Unlock()
}

func (g *Graph) AddAll(ks []Kmer, q []float64) {
	g.Lock()
	for i, k := range ks {
		if n, found := g.kmap[k]; found {
			g.heap[n].n += q[i]
		} else {
			g.kmap[k] = len(g.heap)
			g.heap = append(g.heap, Kpair { k, q[i] })
		}
	}
	g.Unlock()
}

func (g *Graph) Sub(k Kmer, v float64) {
	if n, found := g.kmap[k]; found {
		g.heap[n].n -= v
		heap.Fix(g, n)
	} else {
		panic(fmt.Sprintf("kmer '%v' not found", k))
	}
}

func (g *Graph) Find(k Kmer) float64 {
	if n, found := g.kmap[k]; found {
		return g.heap[n].n
	} else {
		return 0
	}
}

func (g *Graph) Max() (ret Kmer, count float64) {
	if !g.heapinit {
		heap.Init(g)
		g.heapinit = true
	}

	return g.heap[0].k, g.heap[0].n
}

func (g *Graph) BestOligo(maxlen int, minQuality float64) (o *Oligo) {
	// start with the most abundand kmer
	kmer, count := g.Max()
	if count < minQuality {
		return nil
	}

	o = NewOligo(kmer, g.klen, count)
	limit := count / 10

//	fmt.Fprintf(os.Stderr, "Start %v\n", kmer)
	for {
		// find the best kmer to extend to the left
		lkmer := o.lkmer[0:len(o.lkmer) - 1]
		var leftv float64
		leftnt := -1
		for n := 0; n < 4; n++ {
			kk := Kmer(oligo.Nt2String(n)) + lkmer
//			fmt.Fprintf(os.Stderr, "Left %d:%v %d\n", n, kk, g.Find(kk))
			if vv := g.Find(kk); vv > leftv {
				// prevent loops
				if !o.Contains(kk) {
					leftv = vv
					leftnt = n
				}
			} 
		}
//		fmt.Fprintf(os.Stderr, "Best left: %d %d\n", leftnt, leftv)

		// find the best kmer to extend to the right
		rkmer := o.rkmer[1:]
		var rightv float64
		rightnt := -1
		for n := 0; n < 4; n++ {
			kk := rkmer + Kmer(oligo.Nt2String(n))
//			fmt.Fprintf(os.Stderr, "Right %d:%v %d\n", n, kk, g.Find(kk))
			if vv := g.Find(kk); vv > rightv {
				// prevent loops
				if !o.Contains(kk) {
					rightv = vv
					rightnt = n
				}
			} 
		}
//		fmt.Fprintf(os.Stderr, "Best right: %d %d\n", rightnt, rightv)

		if rightv < limit && leftv < limit {
			// couldn't find extension on both ends
			break
		}

		if leftv > rightv {
			o.Prepend(leftnt, g.klen, leftv)
		} else {
			o.Append(rightnt, g.klen, rightv)
		}

		if maxlen != 0 && o.Len() >= maxlen {
			break
		}
	}

	return
}

func (g *Graph) Less(i, j int) bool {
	return g.heap[i].n > g.heap[j].n
}

func (g *Graph) Swap(i, j int) {
	tmp := g.heap[i]
	g.heap[i] = g.heap[j]
	g.heap[j] = tmp

	g.kmap[g.heap[i].k] = i
	g.kmap[g.heap[j].k] = j
}

func (g *Graph) Len() int {
	return len(g.heap)
}


func (g *Graph) Pop() interface{} {
	p := g.heap[0]

	g.heap = g.heap[1:]
	delete(g.kmap, p.k)

	return p
}

func (g *Graph) Push(x interface{}) {
	p := x.(Kpair)
	g.kmap[p.k] = len(g.heap)
	g.heap = append(g.heap, p)
}

func NewOligo(k Kmer, klen int, count float64) *Oligo {
	o := new(Oligo)
	ol, _ := long.FromString(string(k))
	o.ol, _ = long.Copy(ol)
	o.lkmer = k
	o.rkmer = k
	o.mval = count
	o.kmap = make(map[Kmer] bool)
	o.kmap[k] = true

	return o
}

func (o *Oligo) Len() int {
	return o.ol.Len()
}

func (o *Oligo) Append(nt int, klen int, v float64) {
	ol := short.Val(1, uint64(nt))
//	fmt.Fprintf(os.Stderr, "Append %v %v: ", o.ol, ol)
	o.ol.Append(ol)
//	fmt.Fprintf(os.Stderr, "%v\n", o.ol)
//	fmt.Fprintf(os.Stderr, "\told rkmer %v\n", o.rkmer)
	o.rkmer = o.rkmer[1:] + Kmer(oligo.Nt2String(nt))
//	fmt.Fprintf(os.Stderr, "\tnew rkmer %v\n", o.rkmer)
	o.kmap[o.rkmer] = true
	if o.mval > v {
		o.mval = v
	}
}

func (o *Oligo) Prepend(nt int, klen int, v float64) {
	ol := long.New(1)
	ol.Set(0, nt)

//	fmt.Fprintf(os.Stderr, "Prepend %v (%d) %v: ", ol, nt, o.ol)
	ol.Append(o.ol)
	o.ol = ol
//	fmt.Fprintf(os.Stderr, "%v\n", o.ol)

//	fmt.Fprintf(os.Stderr, "\told lkmer %v\n", o.lkmer)
	o.lkmer = Kmer(oligo.Nt2String(nt)) + o.lkmer[0:len(o.lkmer) - 1] 
//	fmt.Fprintf(os.Stderr, "\tnew lkmer %v\n", o.lkmer)
	o.kmap[o.lkmer] = true
	if o.mval > v {
		o.mval = v
	}
}

func (o *Oligo) Contains(k Kmer) bool {
	_, found := o.kmap[k]
	return found
}

func (o *Oligo) Seq() oligo.Oligo {
	return o.ol
}

func (o *Oligo) MinCount() float64 {
	return o.mval
}

func main() {
	var pr5, pr3 oligo.Oligo

	flag.Parse()
	if *p5 != "" {
		var ok bool

		pr5, ok = long.FromString(*p5)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid 5'-end primer: %s\n", *p5)
			return
		}
	}

	if *p3 != "" {
		var ok bool

		pr3, ok = long.FromString(*p3)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid 3'-end primer: %s\n", *p3)
			return
		}
	}

	ch := make(chan Seq, 20)
	ech := make(chan int)
	graph := NewGraph(*klen)
	nprocs := runtime.NumCPU()
	for i := 0; i < nprocs; i++ {
		go seqproc(graph, *klen, ch, ech)
	}

	count := 0
	t := time.Now()
	for i := 0; i < flag.NArg(); i++ {
		fmt.Fprintf(os.Stderr, "\nProcessing %s", flag.Arg(i))
		fname := flag.Arg(i)
		fproc := func(id, sequence string, quality []byte, reverse bool) error {
			if !*useqscore {
				quality = nil
			}

                        ch <- Seq{sequence, quality, reverse}
                        return nil
                }
/*
		fproc := func(id, sequence string, quality []byte, reverse bool) error {
			if count != 0 && count%10000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
			}

			count++
			for i := 0; i + *klen < len(sequence); i++ {
				ol, ok := long.FromString(sequence[i:i+*klen])
				if !ok {
					continue
				}

				if reverse {
					oligo.Reverse(ol)
					oligo.Invert(ol)
				}

				if !criteria.H4G2.Check(ol) {
					continue
				}

				graph.Add(Kmer(ol.String()))
			}

                        return nil
                }
*/

		var err error
		switch (*ftype) {
		default:
			fmt.Fprintf(os.Stderr, "Error: invalid file type: %s\n", *ftype)
			return

		case "csv":
			err = csv.Parse(fname, fproc)

		case "fastq":
			err = fastq.Parse(fname, fproc)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	// signal the coroutines to finish
	for i := 0; i < nprocs; i++ {
		ch <- Seq{"", nil, false}
	}

	// and wait for them to actually finish
	for i := 0; i < nprocs; i++ {
		count += <-ech
	}
	d := time.Since(t)

	k, v := graph.Max()
	fmt.Fprintf(os.Stderr, "\nTotal oligos: %d (%v oligos/s) Number of kmers: %d (%v kmers/s)\nMost abundant: %v %v\n", count,
		float64(count)/float64(d.Seconds()), graph.Len(), float64(graph.Len())/float64(d.Seconds()), k, v)

	if *profname != "" {
		f, err := os.Create(*profname)
		if err != nil {
			fmt.Printf("Error: creating '%s': %v\n", *profname, err)
			return
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Printf("can't start CPU profile: %v\n", err)
			return
		}
		defer pprof.StopCPUProfile()
	}

	var oligos []*Oligo
	var total, good int
	t = time.Now()
	for ; ; total++ {
		bo := graph.BestOligo(*maxlen + (*maxlen / 10), *minqual)		// 10% error
		if bo == nil {
			break
		}

		ol := bo.Seq()
		mval := bo.MinCount()

		for k, _ := range bo.kmap {
			graph.Sub(k, mval)
		}

//		fmt.Printf("%v: %v\n", ol, mval)
		if ol.Len() < (*maxlen - (*maxlen / 10)) {		// 10% error
/*			if total%1000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
				if total%10000 == 0  {
					fmt.Fprintf(os.Stderr, "(%d, %v)", ol.Len(), mval)
				}
			}
*/
			continue
		}

		good++
		if good%1000 == 0{
			fmt.Fprintf(os.Stderr, "+ %v\n", mval)
		}

		if *noligos != 0 && good >= *noligos {
			break
		}

		oligos = append(oligos, bo)
	}
	d = time.Since(t)
	fmt.Fprintf(os.Stderr, "\n")

	sort.Slice(oligos, func (i, j int) bool {
		return oligos[i].mval > oligos[j].mval
	})

	var trimmed []*Oligo
	if pr5 != nil && pr3 != nil {
		for _, ol := range oligos {
			if tol := utils.TrimOligo(ol.ol, pr5, pr3, 3, true); tol != nil {
				ol.ol = tol
				trimmed = append(trimmed, ol)
			}
		}
	} else {
		trimmed = oligos
	}

	for _, ol := range trimmed {
		fmt.Printf("%v %v\n", ol.ol, ol.mval)
	}

	fmt.Fprintf(os.Stderr, "total %d oligos (%v oligos/s), good %d (%v oligos/s) trimmed %d\n", total, float64(total)/float64(d.Seconds()),
		good, float64(good)/float64(d.Seconds()), len(trimmed))
}

func seqproc(graph *Graph, klen int, ch chan Seq, ech chan int) {
	var count  uint64

	kmers := make([]Kmer, 1000)
	qual := make([]float64, 1000)
	for {
		var ol oligo.Oligo
		var ok bool

		s := <-ch
		if s.seq == "" {
			break
		}

		count++
		if count%10000 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}

		ol, ok = long.FromString(s.seq)
		if !ok {
			continue
		}

		if s.rev {
			oligo.Reverse(ol)
			oligo.Invert(ol)

			// reverse the quality array
			for n, i := len(s.quality), 0; i < n/2; i++ {
				s.quality[i], s.quality[n - i - 1] = s.quality[n - i - 1], s.quality[i]
			}
		}

		// generate all kmers from the oligo
		seq := ol.String()
		nkmer := len(seq) - klen + 1
		if nkmer < 0 {
			continue
		}
//		if len(kmers) < nkmer {
//			kmers = make([]Kmer, nkmer)
//		}

		var n int
		for i := 0; i < nkmer; i++ {
			kmer := seq[i:i+klen]
			kol := long.FromString1(kmer)
			if !criteria.H4G2.Check(kol) {
				continue
			}

			kmers[n] = Kmer(kmer)
			qual[n] = 1
			if s.quality != nil {
				for j := i; j < i + klen; j++ {
					qual[n] *= 1 - math.Pow(10, -float64(s.quality[i]) / 10)
				}

//				fmt.Printf("** %v: %v\n", kmers[n], qual[n])
			}

			n++
		}

//		fmt.Printf("%v -> %v\n", ol, kmers[0:nkmer])
		// add them to the map and heap
		graph.AddAll(kmers[0:n], qual[0:n])
	}

	ech <- int(count)
}
