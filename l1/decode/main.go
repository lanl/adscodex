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
	"strings"
	"strconv"
	"sync/atomic"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l1"
	"adscodex/criteria"
)

var crit = flag.String("crit", "h4g2", "criteria")
var dbnum = flag.Int("dbnum", 9, "number of data blocks")
var dbsz = flag.Int("dbsz", 10, "size of a data block in nts")
var dbmindist = flag.Int("dbmindist", 4, "minimum distance between oligos in data blocks")

var mdnum = flag.Int("mdnum", 4, "number of metadata blocks")
var mdsz = flag.Int("mdsz", 10, "metadata block size in nts")
var mdmindist = flag.Int("mdmindist", 4, "minimum distance between oligos in metadata blocks")
var mdcnum = flag.Int("mdcnum", 1, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")

var maxtime = flag.Int64("maxtime", 1000, "maximumm time (in ms) to spend decoding a sequence")

var p5 = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3 = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")

var cdc *l1.Codec
var pr5, pr3 oligo.Oligo
var total uint64

func main() {
	var results []*l1.Entry
	var err error

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

	c := criteria.Find(*crit)
	if c == nil {
		fmt.Printf("Error: invalid criteria\n")
		return
	}

	cdc, err = l1.NewCodec(pr5, pr3, *dbnum, *dbsz, *dbmindist, *mdnum, *mdsz, *mdcnum, *mdmindist, c, *maxtime)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	switch *mdctype {
	default:
		err = fmt.Errorf("Error: invalid metadata EC type")

	case "crc":
		err = cdc.SetMetadataChecksum(l1.CSumCRC)

	case "rs":
		err = cdc.SetMetadataChecksum(l1.CSumRS)
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
