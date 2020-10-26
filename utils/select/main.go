package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/io/fastq"
	"acoma/io/csv"
	"acoma/utils"
)

type Seq struct {
	seq	string
	quality	[]byte
	rev	bool
}

var pdist = flag.Int("pdist", 3, "errors in primer");
var dist = flag.Int("dist", 3, "errors in payload");
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")
var unique = flag.Bool("uq", true, "select unique oligos")
var datasetFile = flag.String("ds", "", "dataset file")
var ftype = flag.String("t", "fastq", "file type")
var oligolen = flag.Int("l", 0, "if not zero, select only oligos with length +/- 10% of the specified value")
var useqscore = flag.Bool("q", true, "use quality score (if available)")

var pr5, pr3 oligo.Oligo
var dspool *utils.Pool
var ulock sync.Mutex
var umap map[string]*utils.Oligo
var total, selected, prcount uint64
var dsmap map[string]bool

func main() {

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


	if *datasetFile != "" {
		var err error

		dspool, err = utils.ReadPool([]string { *datasetFile }, false, csv.Parse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		dspool.Trim(pr5, pr3, *pdist, true)
		if err := dspool.InitSearch(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		dsmap = make(map[string]bool)
		fmt.Fprintf(os.Stderr, "Dataset oligos: %d\n", dspool.Size())
	}

	fmt.Fprintf(os.Stderr, "Distance: %d Primer distance: %d\n", *dist, *pdist)
//	fmt.Fprintf(os.Stderr, "5'-end: %v\n", pr5)
//	fmt.Fprintf(os.Stderr, "3'-end: %v\n", pr3)
	umap = make(map[string] *utils.Oligo)
	ch := make(chan Seq, 20)
	ech := make(chan bool)
	nprocs := runtime.NumCPU()
	for i := 0; i < nprocs; i++ {
		go seqproc(ch, ech)
	}

	for i := 0; i < flag.NArg(); i++ {
		fmt.Fprintf(os.Stderr, "Processing %s\n", flag.Arg(i))
		fname := flag.Arg(i)
		fproc := func(id, sequence string, quality []byte, reverse bool) error {
			if !*useqscore {
				quality = nil
			}

                        ch <- Seq{sequence, quality, reverse}
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

	for i := 0; i < nprocs; i++ {
		ch <- Seq{"", nil, false}
	}

	for i := 0; i < nprocs; i++ {
		<-ech
	}


	if *unique {
		oligos := make([]*utils.Oligo, 0, len(umap))
		for _, o := range umap {
			oligos = append(oligos, o)
		}

		sort.Slice(oligos, func (i, j int) bool {
			return oligos[i].Qubundance() > oligos[j].Qubundance()
		})

		for _, o := range oligos {
			fmt.Printf("%v %v %v\n", o.String(), o.Qubundance(), o.Count())
		}
	}

	dsmatch := 0
	if dspool != nil {
		for _, v := range dsmap {
			if v {
				dsmatch++
			}
		}

		fmt.Fprintf(os.Stderr, "\nTotal: %d, selected %d, with primers %d, dataset matches %d\n", total, selected, prcount, dsmatch)
	} else {
		fmt.Fprintf(os.Stderr, "\nTotal: %d, selected %d, with primers %d\n", total, selected, prcount)
	}
}

func seqproc(ch chan Seq, ech chan bool) {
	var ok bool
	var count, ptotal, prcnt uint64

	qubu := make([]float64, 500)
	for {
		var ol *utils.Oligo

		s := <-ch
		if s.seq == "" {
			break
		}

		count++
		if count%10000 == 0 {
			fmt.Fprintf(os.Stderr, ".")
		}

		ptotal++
		if len(s.quality) > len(qubu) {
			qubu = make([]float64, len(s.quality))
		}

		for i, q := range s.quality {
			qubu[i] = 1 - utils.PhredQuality(q)
		}

		ol, ok = utils.FromString(s.seq, qubu[0:len(s.quality)])
		if !ok {
			continue
		}

		if s.rev {
			ol.Reverse()
			ol.Invert()
		}

		tol1 := ol.Trim(pr5, pr3, *pdist, true)
		if tol1 == nil {
			continue
		}

		tol := tol1.(*utils.Oligo)
		if *oligolen != 0 {
			tlen := float64(tol.Len())
			olen := float64(*oligolen)
			if tlen < olen*0.9 || tlen > olen*1.1 {
				continue
			}
		}

		prcnt++
		if dspool != nil {
			ms := dspool.Search(tol, *dist)
			if ms == nil {
				// doesn't match an oligo in the dataset
				continue
			}

			ulock.Lock()
			for _, m := range ms {
				dsmap[m.Seq.String()] = true
			}
			ulock.Unlock()
		}

		ss := tol.String()
		if *unique {
			ulock.Lock()
			if o, ok := umap[ss]; ok {
				o.Inc(tol.Count(), tol.Qubundances())
			} else {
				umap[ss] = tol
			}
			ulock.Unlock()
		} else {
			fmt.Printf("%v %v %v\n", ss, ol.Qubundance(), ol.Count())
		}
	}

	ulock.Lock()
	total += ptotal
	selected += count
	prcount += prcnt
	ulock.Unlock()

	ech <- true
}
