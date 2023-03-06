package l1

import (
	"flag"
	"fmt"
	"os"
	"math/rand"
	"testing"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/criteria"
)

var mindist = flag.Int("mindist", 4, "minimum distance between oligos in the L0 codec")
var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var dbsz = flag.Int("dbsz", 10, "data block size")
var mdnum = flag.Int("mdnum", 4, "number of metadata blocks")
var mdsz = flag.Int("mdsz", 10, "metadata block size")
var mdcnum = flag.Int("mdcnum", 1, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")
var iternum = flag.Int("iternum", 100, "number of iterations")
var errnum = flag.Int("errnum", 3, "number of errors")
var crit = flag.String("crit", "h4g2", "criteria")
var maxtime = flag.Int64("maxtime", 1000, "maximum time (in ms) to spend decoding a sequence")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

var cdc *Codec
var p5, p3 oligo.Oligo

var failed1 = [...]string {
}

var failed2 = [...]string {
}


func initTest(t *testing.T) {
	var err error

	if cdc != nil {
		return
	}

	c := criteria.Find(*crit)
	if c == nil {
		t.Fatalf("criteria '%s' not found\n", *crit)
	}
	
	p5, _ = long.FromString("CGACATCTCGATGGCAGCAT")
	p3, _ = long.FromString("CAGTGAGCTGGCAACTTCCA")

	cdc, err = NewCodec(p5, p3, *dbnum, *dbsz, *mindist, *mdnum, *mdsz, *mdcnum, *mindist, c, *maxtime)
	if err != nil {
		t.Fatal(err)
	}
	
	switch *mdctype {
	default:
		t.Fatalf("Error: invalid metadata EC type")

	case "crc":
		err = cdc.SetMetadataChecksum(CSumCRC)

	case "rs":
		err = cdc.SetMetadataChecksum(CSumRS)
	}

	if err != nil {
		t.Fatalf("Error: %v\n", err)
	}

//	if err := cdc.SetErrorModel("163.emdl", 100000); err != nil {
//		t.Fatalf("Error Model Error: %v\n", err)
//	}
}

func TestEncode(t *testing.T) {
	initTest(t)

	maxaddr := cdc.MaxAddr()
	fmt.Printf("maxaddr: %d\n", maxaddr)

	data := make([]byte, cdc.DataLen())
	for n := 0; n < *iternum; n++ {
		for i := 0; i < len(data); i++ {
			data[i] = byte(rand.Intn(256))
		}

		addr := uint64(rand.Intn(int(cdc.MaxAddr() - 2)))
		ec := n%2 == 0
		tt := time.Now()
		ol, err := cdc.Encode(addr, ec, data)
		et := time.Since(tt)
		if err != nil {
			t.Fatalf("error while encoding: %v\n", err)
		}

		tt = time.Now()
		daddr, dec, ddata, _, err := cdc.Decode(ol)
		dt := time.Since(tt)
		if err != nil {
			t.Fatalf("error while decoding: %v\n", err)
		}

		if addr != daddr {
			t.Fatalf("addresses don't match: %v %v\n", addr, daddr)
		}

		if ec != dec {
			t.Fatalf("erasure flag doesn't match: %v %v\n", ec, dec)
		}

		for i := 0; i < len(data); i++ {
			if data[i] != ddata[i] {
				t.Fatalf("data doesn't match: %v %v\n", data, ddata)
			}
		}

//		fmt.Printf("addr %d ec %v dblks %v oligo: %v: %v %v %v\n", addr, ec, dblks, ol, et, dt, oligo.GCcontent(ol))
		fmt.Printf("oligo: %v: %v %v %v\n", ol, et, dt, oligo.GCcontent(ol))

	}
}

/*
func TestRecover2(t *testing.T) {
	return

	initTest(t)

	for _, s := range failed2 {
		ol, _ := long.FromString(s)
		fmt.Printf("%v\n", ol)
		cdc.Decode(p5, p3, ol, 1)

//		fmt.Printf("%v: %v %v %v %v\n", ol, addr, ec, data, err)
	}
}


func TestRecover(t *testing.T) {
	initTest(t)

	nerr := *errnum
	niter := *iternum

	blks := make([][]byte, cdc.BlockNum())
	for i := 0; i < len(blks); i++ {
		blks[i] = make([]byte, cdc.BlockSize())
	}

	errnum := 0
	errpositive := 0
	for n := 0; n < niter; n++ {
		for i := 0; i < len(blks); i++ {
			for j := 0; j < len(blks[i]); j++ {
				blks[i][j] = byte(rand.Intn(256))
			}
		}

		addr := uint64(rand.Intn(int(cdc.MaxAddr() - 2)))
		ec := n%2 == 0
//		tt := time.Now()
		ol, err := cdc.Encode(p5, p3, addr, ec, blks)
//		et := time.Since(tt)
		if err != nil {
			t.Fatalf("error while encoding: %v\n", err)
		}

		// add some errors
		seq := ol.String()
		for i := 0; i < nerr; i++ {
			idx := rand.Intn(len(seq) - 1)
			switch rand.Intn(3) {
			case 0:
				// delete
				seq = seq[0:idx] + seq[idx+1:]

			case 1:
				// insert
				seq = seq[0:idx] + oligo.Nt2String(rand.Intn(4)) + seq[idx:]

			case 2:
				// replace
				seq = seq[0:idx] + oligo.Nt2String(rand.Intn(4)) + seq[idx+1:]
			}
		}
		eol, _ := long.FromString(seq)
		
//		tt = time.Now()
		daddr, dec, _, err := cdc.Decode(p5, p3, eol, *dfclty)
//		dt := time.Since(tt)

		if err != nil {
			errnum++
		}

		if err == nil && (addr != daddr || ec != dec) {
			errpositive++
		}

//		fmt.Printf("orig: %v\nerr:  %v %v %v\n", ol, eol, et, dt)

	}

//	fmt.Printf("error rate %v false positive rate %v\n", float64(errnum)/float64(niter), float64(errpositive)/float64(niter))
	fmt.Printf("%d %d %d %s %d %v %v\n", *dbnum, *mdsz, *mdcnum, *mdctype, *dfclty, float64(errnum)/float64(niter), float64(errpositive)/float64(niter))
}

func TestRecover3(t *testing.T) {
	return

	initTest(t)

	addr := uint64(776854)
	ec := true
	data := [][]byte { { 154, 0, 250, 51, }, { 18, 203, 161, 93, }, { 195, 108, 106, 133, }, { 191, 162, 73, 60, }, { 4, 22, 210, 242, },}
	cdc.Encode(p5, p3, addr, ec, data)

	eol, _ := long.FromString("CGACATCTCGATGGCAGCATATGGTGTCAGTAACTGTGTCATTAGCAGACCACGACTACCCGATATTACTGGAAGAGAAGTTTGCGACTACCTTAGTCCCTGCCGTACTTTCGCGTAGTGTAGATATGGCAGTGAGCTGGCAACTTCCA")
	cdc.Decode(p5, p3, eol, 1)
}
*/
