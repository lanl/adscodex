package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"adscodex/l0"
	"adscodex/oligo/short"
)

func main() {
	var olen int
	var tbl []uint64
	var err error

	flag.Parse()
	if flag.NArg() == 1 {
		olen, tbl, err = l0.ReadTable(flag.Arg(0))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	} else {
		olen, tbl, err = readCsv(flag.Arg(0))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		err = l0.WriteTable(olen, tbl, flag.Arg(1))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	}

	fmt.Printf("Oligo length: %d, Max value: %v\n", olen, len(tbl))

	return
}

func readCsv(csvFile string) (olen int, tbl []uint64, err error) {
	var f *os.File
	var r io.Reader
	var n int

	f, err = os.Open(csvFile)
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
		line := sc.Text()
		if line == "" {
			continue
		}

		ls := strings.Split(line, " ")
		if len(ls) == 1 {
			// support both space-separated and comma-separated
			ls = strings.Split(line, ",")
		}

		ol, ok := short.FromString(ls[0])
		if !ok {
			err = fmt.Errorf("%d: invalid oligo: %v\n", n, ls[0])
			return
		}

		if olen == 0 {
			olen = ol.Len()
		} else if olen != ol.Len() {
			err = fmt.Errorf("%d: oligos of different size: %d: %d", n, olen, ol.Len())
			return
		}

		tbl = append(tbl, ol.Uint64())
	}

	return
}
