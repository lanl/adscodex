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
	rev	bool
}

var pdist = flag.Int("pdist", 3, "errors in primer");
var dist = flag.Int("dist", 3, "errors in payload");
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")
var unique = flag.Bool("uq", true, "select unique oligos")
var datasetFile = flag.String("ds", "", "dataset file")
var ftype = flag.String("t", "fastq", "file type")

var pr5, pr3 oligo.Oligo
var dspool *utils.Pool
var ulock sync.Mutex
var umap map[string]int
var total, selected, prcount uint64

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
	}

	umap = make(map[string] int)
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
                        ch <- Seq{sequence, reverse}
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
		ch <- Seq{"", false}
	}

	for i := 0; i < nprocs; i++ {
		<-ech
	}


	if *unique {
		seqs := make([]string, 0, len(umap))
		for seq, _ := range umap {
			seqs = append(seqs, seq)
		}

		sort.Slice(seqs, func (i, j int) bool {
			return umap[seqs[i]] > umap[seqs[j]]
		})

		for _, seq := range seqs {
			fmt.Printf("%v %d\n", seq, umap[seq])
		}
	}

	fmt.Fprintf(os.Stderr, "\nTotal: %d, selected %d, with primers %d\n", total, selected, prcount)
}

func seqproc(ch chan Seq, ech chan bool) {
	var count, ptotal, prcnt uint64

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

		ptotal++
		ol, ok = long.FromString(s.seq)
		if !ok {
			continue
		}

		if s.rev {
			oligo.Reverse(ol)
			oligo.Invert(ol)
		}

		if !utils.TrimOligo(ol, pr5, pr3, *pdist, true) {
			continue
		}

		prcnt++
		if dspool != nil && dspool.Search(ol, *dist) == nil {
			// doesn't match an oligo in the dataset
			continue
		}

		ss := ol.String()
		if *unique {
			ulock.Lock()
			umap[ss]++
			ulock.Unlock()
		} else {
			fmt.Printf("%v\n", ss)
		}
	}

	ulock.Lock()
	total += ptotal
	selected += count
	prcount += prcnt
	ulock.Unlock()

	ech <- true
}
