package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"runtime/pprof"
	"adscodex/criteria"
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

var dbnum = flag.Int("dbnum", 9, "number of data blocks")
var dbsz = flag.Int("dbsz", 10, "size of a data block in nts")
var dbmindist = flag.Int("dbmindist", 4, "minimum distance between oligos in data blocks")

var mdnum = flag.Int("mdnum", 4, "number of metadata blocks")
var mdsz = flag.Int("mdsz", 10, "metadata block size in nts")
var mdmindist = flag.Int("mdmindist", 4, "minimum distance between oligos in metadata blocks")
var mdcnum = flag.Int("mdcnum", 1, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")

var maxtime = flag.Int64("maxtime", 1000, "maximumm time (in ms) to spend decoding a sequence")

var dseqnum = flag.Int("dseqnum", 3, "number of data oligos per erasure group")
var rseqnum = flag.Int("rseqnum", 2, "number of erasure oligos per erasure group")

var profname = flag.String("prof", "", "profile filename")
var ftype = flag.String("ftype", "csv", "input file type")
var rndomize = flag.Bool("rndmz", false, "randomze data")
var verbose = flag.Bool("v", false, "verbose")
var start = flag.Uint64("addr", 0, "start address")
var end = flag.Uint64("endaddr", math.MaxUint64, "end address")
var tblpath = flag.String("tbl", "", "path to the tables")

func main() {
	flag.Parse()

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

	if *tblpath != "" {
		l0.SetLookupTablePath(*tblpath)
	}

	cdc, err := l2.NewCodec(p5, p3, *dbnum, *dbsz, *dbmindist, *mdnum, *mdsz, *mdmindist, *mdcnum, *dseqnum, *rseqnum, *maxtime, criteria.H4G2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if flag.NArg() != 2 {
		fmt.Printf("Expecting file name\n");
		return
	}

	var mc int
	switch  *mdctype {
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

	cdc.SetRandomize(*rndomize)
	cdc.SetVerbose(*verbose)

	var oligos []oligo.Oligo
	var entries []*l1.Entry

	switch (*ftype) {
	default:
		err = fmt.Errorf("Unsupported input type: %s\n", *ftype)

	case "csv":
		oligos, err = csv.Read(flag.Arg(0), false)

	case "fastq":
		oligos, err = fastq.Read(flag.Arg(0), false)

	case "l1dec":
		entries, err = l1.ReadEntries(flag.Arg(0))
		fmt.Fprintf(os.Stderr, "%d entries\n", len(entries))
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

	var data []l2.DataExtent
	if oligos != nil {
		data = cdc.Decode(*start, *end, oligos)
	} else {
		data = cdc.DecodeL1(*start, *end, entries)
	}

	of, err := os.Create(flag.Arg(1))
	if err != nil {
		fmt.Printf("Error creating the file: %s: %v\n", flag.Arg(1), err)
		return
	}

	var vsz, usz, bsz, hsz, off uint64
	for i := 0; i < len(data); i++ {
		d := &data[i]
		if d.Offset != off {
			if d.Offset < off {
				panic(fmt.Sprintf("d.Offset %d off %d\n", d.Offset, off))
			}

			hsz += d.Offset - off
		}

		l := uint64(len(d.Data))
		switch d.Type {
		case l2.FileVerified:
			vsz += l

		case l2.FileUnverified:
			usz += l

		case l2.FileBestGuess:
			bsz += l
		}

		off = d.Offset + uint64(len(d.Data))
//		fmt.Printf("%d: %d verified %v\n", d.Offset, len(d.Data), d.Verified)
		of.Seek(int64(d.Offset), 0)
		of.Write(d.Data)
	}
	of.Close()

	if len(data) != 1 {
		fmt.Fprintf(os.Stderr, "Warning: not all data was recovered, the file has holes\n")
	}

	fmt.Fprintf(os.Stderr, "%d bytes verified, %d unverified, %d best guess %d holes\n", vsz, usz, bsz, hsz)
}
