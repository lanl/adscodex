package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"adscodex/io/csv"
	"adscodex/io/fastq"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/utils"
)

var distance = flag.Int("dist", 3, "match distance")
var pdist = flag.Int("pdist", 3, "allowed errors in primer")
var datasetFile = flag.String("ds", "", "synthesis dataset file")
var debug = flag.Bool("debug", false, "debug")
var p5 = flag.String("p5", "", "5'-end primer")
var p3 = flag.String("p3", "", "3'-end primer")
var ftype = flag.String("ft", "fastq", "file type (csv or fastq)")

type ClusterStat struct {
	cid		int
	diameter	int
	seqs		[]oligo.Oligo
	smatch		[]oligo.Oligo
	readnum		int
}

func main() {
	var err error
	var pool, dspool *utils.Pool
	var pr5, pr3 oligo.Oligo

	flag.Parse()

	if *datasetFile != "" {
		dspool, err = utils.ReadPool([]string { *datasetFile }, false, csv.Parse)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
	}

	var  fns []string
	for i := 0; i < flag.NArg(); i++ {
		fns = append(fns, flag.Arg(i))
	}

	fmt.Fprintf(os.Stderr, "Reading files %v...\n", fns)
	switch *ftype {
	default:
		err = fmt.Errorf("invalid file type: %v\n", *ftype)

	case "csv":
		pool, err =  utils.ReadPool(fns, true, csv.Parse)

	case "fastq":
		pool, err =  utils.ReadPool(fns, true, fastq.Parse)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

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

	pool.Trim(pr5, pr3, *pdist, true)
	if dspool != nil {
		dspool.Trim(pr5, pr3, *pdist, true)
	}

	fmt.Fprintf(os.Stderr, "Building clusters...\n");
	clusters := FindClusters(pool, *distance)
	if *debug {
		fmt.Fprintf(os.Stderr, "# clusters: dist %d %d\n", *distance, len(clusters))
	}

	var clist []ClusterStat
	dspool.InitSearch()
	pchan := make(chan *Cluster)
	rchan := make(chan ClusterStat)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				cl := <- pchan
				if cl == nil {
					return
				}

				sfmap := make(map[oligo.Oligo]bool)
				readnum := 0
				sort.Slice(cl.Oligos, func (i, j int) bool {
					s1 := cl.Oligos[i]
					s2 := cl.Oligos[j]
					return pool.Count(s1) > pool.Count(s2)
				})

				for _, seq := range cl.Oligos {
					readnum += pool.Count(seq)

					if dspool != nil {
						matches := dspool.Search(seq, *distance)
						for _, m := range matches {
							sfmap[m.Seq] = true
						}
					}
				}

				var smatch []oligo.Oligo
				for f, _ := range sfmap {
					smatch = append(smatch, f)
				}

				rchan <- ClusterStat { 0, cl.Diameter, cl.Oligos, smatch, readnum }
			}
		}()
	}

	seqnum := 0
	for i, n := 0, 0; i < 2*len(clusters); i++ {
		var c *Cluster

		if n < len(clusters) {
			c = clusters[n]
		} else {
			pchan = nil
		}

//		fmt.Fprintf(os.Stderr, "%d %d  %p\n", i, n, pchan)
		select {
		case pchan <- c:
//			fmt.Fprintf(os.Stderr, ">>> %p\n", c)
			n++

		case cs := <- rchan:
//			fmt.Fprintf(os.Stderr, "<<< %p\n", &cs)
			cs.cid = len(clist)
			clist = append(clist, cs)
			seqnum += len(cs.seqs)
		}
	}

	// send some more nils to the goroutines
	for i := 0; i < runtime.NumCPU(); i++ {
		select {
		default:
		case pchan <- nil:
		}
	}

	sort.Slice(clist, func(i, j int) bool {
		r1 := 0
		for _, s := range clist[i].seqs {
			r1 += pool.Count(s)
		}

		r2 := 0
		for _, s := range clist[j].seqs {
			r2 += pool.Count(s)
		}

		return r1 > r2
	})

	for _, cs := range clist {
		readnum := 0
		for _, s := range cs.seqs {
			readnum += pool.Count(s)
		}

		fmt.Fprintf(os.Stderr, "%d %d %d %d %d\n", cs.cid, cs.diameter, len(cs.seqs), len(cs.smatch), readnum)
		if *debug {
			for _, s := range cs.smatch {
				fmt.Fprintf(os.Stdout, "\t#SYN %s\n", s)
			}

			for _, s := range cs.seqs {
				fmt.Fprintf(os.Stdout, "\t#SEQ %s %d\n", s, pool.Count(s))
			}
		}
	}
}
