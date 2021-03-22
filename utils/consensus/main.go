package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
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

type Kmer uint64
type Kpair struct {
	k	Kmer
	n	int
}

type Graph struct {
	klen	int		// kmer length
	kmask	Kmer
	kshift	int

	kmap	map[Kmer]int
	heap	[]Kpair
	heapinit bool
}

type Oligo struct {
	ol	*long.Oligo
	lkmer	Kmer
	rkmer	Kmer
	mval	int
	kmap	map[Kmer] bool
}

var ftype = flag.String("t", "fastq", "file type")
var klen = flag.Int("k", 31, "kmer length")
var profname = flag.String("prof", "", "profile filename")
var noligos = flag.Int("n", 0, "number of oligos")
var maxlen = flag.Int("l", 0, "maximum oligo length")
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")

func NewGraph(kmerLen int) *Graph {
	g := new(Graph)
	g.kmap = make(map[Kmer]int)
	g.klen = kmerLen
	g.kshift = 2*kmerLen
	g.kmask = (1<<g.kshift) - 1
	g.heap = nil

	return g
}

func (g *Graph) Add(k Kmer) {
	if n, found := g.kmap[k]; found {
		g.heap[n].n++
	} else {
		g.kmap[k] = len(g.heap)
		g.heap = append(g.heap, Kpair { k, 1 })
	}
}

func (g *Graph) Sub(k Kmer, v int) {
	if n, found := g.kmap[k]; found {
		g.heap[n].n -= v
		heap.Fix(g, n)
	} else {
		panic(fmt.Sprintf("kmer '%v' not found", short.Val(g.klen, uint64(k))))
	}
}

func (g *Graph) Find(k Kmer) int {
	if n, found := g.kmap[k]; found {
		return g.heap[n].n
	} else {
		return 0
	}
}

func (g *Graph) Max() (ret Kmer, count int) {
	if !g.heapinit {
		heap.Init(g)
		g.heapinit = true
	}

	return g.heap[0].k, g.heap[0].n
}

func (g *Graph) BestOligo(maxlen int) (o *Oligo) {
	// start with the most abundand kmer
	kmer, count := g.Max()
	if count == 0 {
		return nil
	}

	o = NewOligo(kmer, g.klen, count)

//	fmt.Fprintf(os.Stderr, "Start %v\n", short.Val(g.klen, uint64(kmer)))
	for {
		// find the best kmer to extend to the left
		lkmer := o.lkmer >> 2
		var leftv int
		leftnt := -1
		for n := 0; n < 4; n++ {
			kk := lkmer | Kmer(n << (g.kshift - 2))
//			fmt.Fprintf(os.Stderr, "Left %d:%v %d\n", n, short.Val(g.klen, uint64(kk)), g.Find(kk))
			if vv := g.Find(kk); vv > leftv {
				// prevent loops
				if !o.Contains(kk) {
					leftv =  vv
					leftnt = n
				}
			} 
		}

		// find the best kmer to extend to the right
		rkmer := (o.rkmer << 2) & g.kmask
		var rightv int
		rightnt := -1
		for n := 0; n < 4; n++ {
			kk := rkmer | Kmer(n)
//			fmt.Fprintf(os.Stderr, "Right %d:%v %d\n", n, short.Val(g.klen, uint64(kk)), g.Find(kk))
			if vv := g.Find(kk); vv > rightv {
				// prevent loops
				if !o.Contains(kk) {
					rightv = vv
					rightnt = n
				}
			} 
		}

		if rightv == 0 && leftv == 0 {
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

func NewOligo(k Kmer, klen int, count int) *Oligo {
	o := new(Oligo)
	ol := short.Val(klen, uint64(k))
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

func (o *Oligo) Append(nt int, klen int, v int) {
	ol := short.Val(1, uint64(nt))
//	fmt.Fprintf(os.Stderr, "Append %v %v: ", o.ol, ol)
	o.ol.Append(ol)
//	fmt.Fprintf(os.Stderr, "%v\n", o.ol)
//	fmt.Fprintf(os.Stderr, "\told rkmer %v\n", short.Val(klen, uint64(o.rkmer)))
	o.rkmer <<= 2
	o.rkmer &= Kmer((1<<(2*klen)) - 1)
	o.rkmer |= Kmer(nt)
//	fmt.Fprintf(os.Stderr, "\tnew rkmer %v\n", short.Val(klen, uint64(o.rkmer)))
	o.kmap[o.rkmer] = true
	if o.mval > v {
		o.mval = v
	}
}

func (o *Oligo) Prepend(nt int, klen int, v int) {
	ol := long.New(1)
	ol.Set(0, nt)

//	fmt.Fprintf(os.Stderr, "Prepend %v %v: ", ol, o.ol)
	ol.Append(o.ol)
	o.ol = ol
//	fmt.Fprintf(os.Stderr, "%v\n", o.ol)

//	fmt.Fprintf(os.Stderr, "\told lkmer %v\n", short.Val(klen, uint64(o.lkmer)))
	o.lkmer = (o.lkmer >> 2) | (Kmer(nt) << (2*(klen - 1)))
//	fmt.Fprintf(os.Stderr, "\tnew lkmer %v\n", short.Val(klen, uint64(o.lkmer)))
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

func (o *Oligo) MinCount() int {
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

	count := 0
	graph := NewGraph(*klen)
	t := time.Now()
	for i := 0; i < flag.NArg(); i++ {
		fmt.Fprintf(os.Stderr, "\nProcessing %s", flag.Arg(i))
		fname := flag.Arg(i)
		fproc := func(id, sequence string, quality []byte, reverse bool) error {
			if count != 0 && count%10000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
			}

			count++
			for i := 0; i + *klen < len(sequence); i++ {
				ol, ok := short.FromString(sequence[i:i+*klen])
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

				graph.Add(Kmer(ol.Uint64()))
			}

                        return nil
                }

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
	d := time.Since(t)

	k, v := graph.Max()
	fmt.Fprintf(os.Stderr, "\nTotal oligos: %d (%v oligos/s) Number of kmers: %d (%v kmers/s)\nMost abundant: %v %d\n", count, 
		float64(count)/float64(d.Seconds()), graph.Len(), float64(graph.Len())/float64(d.Seconds()), short.Val(*klen, uint64(k)), v)

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

	var oligos []oligo.Oligo
	var total, good int
	t = time.Now()
	for ; ; total++ {
		bo := graph.BestOligo(*maxlen)
		if bo == nil {
			break
		}

		ol := bo.Seq()
		mval := bo.MinCount()

		for k, _ := range bo.kmap {
			graph.Sub(k, mval)
		}

//		fmt.Printf("%v: %d\n", ol, mval)
		if ol.Len() < 147 {
//			fmt.Fprintf(os.Stderr, ".")
			continue
		}

		good++
		if good%1000 == 0{
			fmt.Fprintf(os.Stderr, "+")
		}

		if *noligos != 0 && good >= *noligos {
			break
		}

		oligos = append(oligos, ol)
	}
	d = time.Since(t)
	fmt.Fprintf(os.Stderr, "\n")

	pool := utils.NewPool(oligos, true)
	if pr5 != nil && pr3 != nil {
		pool.Trim(pr5, pr3, 3, true)
	}

	for _, o := range pool.Oligos() {
		fmt.Printf("%v\n", o)
	}

	fmt.Fprintf(os.Stderr, "total %d oligos (%v oligos/s), good %d (%v oligos/s)\n", total, float64(total)/float64(d.Seconds()), 
		good, float64(good)/float64(d.Seconds()))
}

