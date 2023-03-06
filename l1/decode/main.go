package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l1"
)

var tblname = flag.String("tbl", "../../tbl/32-10.tbl", "table name")
var maxtime = flag.Int64("maxtime", 5000, "maximumm time (in ms) to spend decoding a sequence")

var p5 = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3 = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")

var cdc *l1.Codec
var pr5, pr3 oligo.Oligo

func main() {
	var results []*l1.Entry
	var err error
	var total uint64

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

	cdc, err = l1.NewCodec(pr5, pr3, *tblname, *maxtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	ols, cntmap, err := csvRead(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}


	ch := make(chan string)
	outch := make(chan []*l1.Entry)
	nprocs := runtime.NumCPU()
	for i := 0; i < nprocs; i++ {
		go func() {
			var ret []*l1.Entry

			for {
				seq := <- ch
				if seq == "" {
					break
				}

				ol, ok := long.FromString(seq)
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid oligo: %s\n", seq)
					continue
				}

				addr, ec, data, errdist, err := cdc.Decode(ol)
				if err != nil {
					continue
				}

				fmt.Fprintf(os.Stderr, "%d *** %d %v %d %v\n", atomic.AddUint64(&total, 1), addr, ec, errdist, data)
				ret = append(ret, &l1.Entry{addr, ec, errdist, data, cntmap[seq]})
			}

			outch <- ret
		}()
	}

	for _, ol := range ols {
		ch <- ol
	}

	// signal end of stream
	for i := 0; i < nprocs; i++ {
		ch <- ""
		r := <- outch
		results = append(results, r...)
	}

	sort.Slice(results, func (i, j int) bool {
		if results[i].Dist < results[j].Dist {
			return true
		} else if results[i].Dist == results[j].Dist {
			return results[i].Addr < results[j].Addr
		}

		return false
	})

	var fname string
	if flag.NArg() == 2 {
		fname = flag.Arg(1)
	}

	err = l1.WriteEntries(fname, results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

// Reads csv file produced by utils/select
// Format is "seq qubu count ..."
func csvRead(fname string) (ols []string, cntmap map[string]int, err error) {
	var f *os.File
	var r io.Reader
	var n int
	var v64 uint64

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

	cntmap = make(map[string]int)

	sc := bufio.NewScanner(r)
	n = 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if line == "" {
			continue
		}

		ls := strings.Split(line, " ")
		if len(ls) < 3 {
			err = fmt.Errorf("%d: invalid line: %s", n, line)
			return
		}

		seq := ls[0]
		v64, err = strconv.ParseUint(ls[2], 10, 32)
		if err != nil {
			err = fmt.Errorf("%d: invalid count: %v: %v", n, ls[2], err)
			return
		}

		cntmap[seq] = int(v64)
		ols = append(ols, seq)
	}

	return
}
