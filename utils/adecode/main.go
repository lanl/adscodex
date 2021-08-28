package main

import (
	"flag"
	"fmt"
	"math"
	"sort"
	"os"
	"runtime/pprof"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l0"
	"adscodex/l1"
	"adscodex/l2"
	"adscodex/io/csv"
	"adscodex/io/fastq"
)

var p5str = flag.String("p5", "CGACATCTCGATGGCAGCAT", "5'-end primer")
var p3str = flag.String("p3", "CAGTGAGCTGGCAACTTCCA", "3'-end primer")
var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block size")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")
var profname = flag.String("prof", "", "profile filename")
var ftype = flag.String("ftype", "csv", "input file type")
var mdcsum = flag.String("mdcsum", "crc", "L1 metadata blocks checksum type (rs for Reed-Solomon, crc for CRC)")
var dtcsum = flag.String("dtcsum", "parity", "L1 data blocks checksum type (parity or even)")
var compat = flag.Bool("compat", false, "compatibility with 0.9")
var rndomize = flag.Bool("rndmz", false, "randomze data")
var ds = flag.String("ds", "", "dataset file")

func main() {
	flag.Parse()

	l0.SetLookupTablePath("../../tbl")

	p5, ok := long.FromString(*p5str)
	if !ok {
		fmt.Printf("Invalid 5'-end primer\n")
		return
	}

	p3, ok := long.FromString(*p3str)
	if !ok {
		fmt.Printf("Invalid 3'-end primer\n")
		return
	}

	cdc, err := l2.NewCodec(p5, p3, *dbnum, *mdsz, *mdcnum, *dseqnum, *rseqnum)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	cdc.SetCompat(*compat)
	cdc.SetSimpleErrorModel(0.012, 0.044, 0.027, 6)

	if flag.NArg() != 2 {
		fmt.Printf("Expecting file name\n");
		return
	}

	var mc, dc int
	switch  *mdcsum {
	default:
		fmt.Printf("Invalid metadata checksum type\n")
		return

	case "rs":
		mc = l1.CSumRS

	case "crc":
		mc = l1.CSumCRC
	}

	if err := cdc.SetMetadataChecksum(mc); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	switch  *dtcsum {
	default:
		fmt.Printf("Invalid data checksum type: %s\n", *dtcsum)
		return

	case "parity":
		dc = l1.CSumParity

	case "even":
		dc = l1.CSumEven
	}

	if err := cdc.SetDataChecksum(dc); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	cdc.SetRandomize(*rndomize)

	dsoligos, err := csv.Read(*ds, true)
	if err != nil {
		fmt.Printf("Can't parse dataset file: %v\n", err)
		return
	}

	/*dsdata*/ _, dsrecs := cdc.DecodeVerbose(0, math.MaxUint64, dsoligos)
	sort.Slice(dsrecs, func(i, j int) bool {
		return dsrecs[i].Addr < dsrecs[j].Addr
	})

	var oligos []oligo.Oligo
	switch (*ftype) {
	default:
		err = fmt.Errorf("Unsupported input type: %s\n", *ftype)

	case "csv":
		oligos, err = csv.Read(flag.Arg(0), true)

	case "fastq":
		oligos, err = fastq.Read(flag.Arg(0), true)
	}

	if err != nil {
		fmt.Printf("Can't  parse input: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "%d oligos\n", len(oligos))
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

	data, recs := cdc.DecodeVerbose(0, math.MaxUint64, oligos)
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Addr < recs[j].Addr
	})

	di := 0
	ecgrp := uint64(0)
	nidx := 0
	prevaddr := uint64(math.MaxUint64)
	exts := ""
	ver := ""
	if nidx < len(data) {
		exts = fmt.Sprintf("(%d, %d, %v) ", data[nidx].Offset, data[nidx].Offset + uint64(len(data[nidx].Data)), data[nidx].Type)
	}
	for i := 0; i < len(recs); i++ {
		r := &recs[i]
		if r.Addr == math.MaxUint64 {
			continue
		}

		if r.Ecgrp != ecgrp {
			ecgrp = r.Ecgrp

			fmt.Printf("*** %s %s\n", ver, exts)
			ver = ""
			if nidx < len(data) {
				exts = fmt.Sprintf("(%d, %d, %v) ", data[nidx].Offset, data[nidx].Offset + uint64(len(data[nidx].Data)), data[nidx].Type)
			} else {
				exts = ""
			}

		}

		// find the matching record from the dataset
		var dr *l2.DecRecord
		for ; di < len(dsrecs); di++ {
			dr = &dsrecs[di]
			if dr.Addr < r.Addr {
				continue
			} else if dr.Addr == r.Addr {
				break
			} else if dr.Addr > r.Addr {
				dr = nil
				break
			}
		}

		if dr == nil {
			// no matching record from the dataset, just print it (if it recovered any data)
			dnum := 0
			for _, d := range r.Data {
				if d != nil {
					dnum++
				}
			}

			if dnum > 0 {
				fmt.Printf("%d %d %d %d %v %v %d %d\n", r.Addr, r.Ecgrp, r.Ecrow, r.Lvl, r.Data, r.Ol, r.Oaddr, r.Offset)
			}

			continue
		}

		match := ""
		for j := 0; j < len(r.Data); j++ {
			m := "-"
			d := r.Data[j]
			dds := dr.Data[j]

			if d == nil {
				m = "H"
			} else {
				for l := 0; l < len(d); l++ {
					if d[l] != dds[l] {
						m = "X"
						break
					}
				}
			}

			match += m
		}

		fmt.Printf("%d %d %d %d %v %v %d %d\n", r.Addr, r.Ecgrp, r.Ecrow, r.Lvl, match, r.Ol, r.Oaddr * int64(*dbnum * 4), r.Offset)

		if r.Addr == prevaddr {
			continue
		}
		if prevaddr != math.MaxUint64 {
			for i := 0; i < int(r.Addr - prevaddr - 1) * *dbnum; i++ {
				ver += "-"
			}
		}
		prevaddr = r.Addr

		if r.Offset < 0 {
			for i := 0; i < *dbnum; i++ {
				ver += "*"
			}

			continue
		}

		for i := 0; i < *dbnum; i++ {
			off := uint64(r.Offset) + uint64(i * 4)

			for nidx < len(data) {
				if off < data[nidx].Offset + uint64(len(data[nidx].Data)) {
					break
				}

				nidx++
				if nidx < len(data) {
					exts += fmt.Sprintf("(%d, %d, %v) ", data[nidx].Offset, data[nidx].Offset + uint64(len(data[nidx].Data)), data[nidx].Type)
				}
			}

			if nidx < len(data) {
				de := &data[nidx]
				dend := de.Offset + uint64(len(de.Data))
				if off >= de.Offset && off < dend {
					if de.Type == l2.FileVerified {
						ver += "V"
					} else if de.Type == l2.FileUnverified {
						ver += "U"
					} else if de.Type == l2.FileBestGuess {
						ver += "G"
					} else {
						ver += "?"
					}
				} else {
					ver += "H"
				}
			}
		}
	}
}
