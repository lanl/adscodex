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
	"adscodex/l0"
//	"adscodex/criteria"
)

var dbnum = flag.Int("dbnum", 7, "number of data blocks")
var mdnum = flag.Int("mdnum", 3, "number of metadata blocks")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "crc", "metadata error detection type (rs or crc)")
var iternum = flag.Int("iternum", 100, "number of iterations")
var errnum = flag.Int("errnum", 3, "number of errors")
var dfclty =  flag.Int("dfclty", 1, "decoding difficulty level")
var crit = flag.String("crit", "h4g2", "criteria")
var tbl = flag.String("tbl", "../tbl/165o6b8.tbl", "codec table")

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
	if cdc != nil {
		return
	}

	p5, _ = long.FromString("CGACATCTCGATGGCAGCAT")
	p3, _ = long.FromString("CAGTGAGCTGGCAACTTCCA")

	c0, err := l0.Load(*tbl)
	if err != nil {
		t.Fatal(err)
	}

	cdc, err = NewCodec(*dbnum, *mdnum, *mdcnum, c0)
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
}

func TestEncode(t *testing.T) {
	return

	initTest(t)

	maxaddr := cdc.MaxAddr()
	fmt.Printf("maxaddr: %d\n", maxaddr)

	blks := make([][]byte, cdc.BlockNum())
	for n := 0; n < 200; n++ {
		for j := 0; j < len(blks); j++ {
			blks[j] = make([]byte, cdc.BlockSize())
			for i := 0; i < len(blks[j]); i++ {
				blks[j][i] = byte(rand.Intn(256))
			}
		}

		addr := uint64(rand.Intn(int(cdc.MaxAddr() - 2)))
		ec := n%2 == 0
		tt := time.Now()
		ol, err := cdc.Encode(p5, p3, addr, ec, blks)
		et := time.Since(tt)
		if err != nil {
			t.Fatalf("error while encoding: %v\n", err)
		}

		fmt.Printf(">>> %v\n", ol)
		tt = time.Now()
		daddr, dec, dblks, err := cdc.Decode(p5, p3, ol, 0)
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

		for i := 0; i < len(blks); i++ {
			for j, b := range blks[i] {
				if b != dblks[i][j] {
					t.Fatalf("data doesn't match: %v %v\n", blks, dblks)
				}
			}
		}

//		fmt.Printf("addr %d ec %v dblks %v oligo: %v: %v %v %v\n", addr, ec, dblks, ol, et, dt, oligo.GCcontent(ol))
		fmt.Printf("oligo: %v: %v %v %v\n", ol, et, dt, oligo.GCcontent(ol))

	}
}

func TestRecover(t *testing.T) {
	initTest(t)

	nerr := *errnum
	niter := *iternum

	blks := make([][]byte, cdc.BlockNum())
	errnum := 0
	errpositive := 0
	for n := 0; n < niter; n++ {
		for i := 0; i < len(blks); i++ {
			blks[i] = make([]byte, cdc.BlockSize())
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

//		fmt.Printf(">>> %v\n", ol)

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

//		fmt.Printf("--- %v\n", eol)		
//		tt = time.Now()
		daddr, dec, dblks, err := cdc.Decode(p5, p3, eol, *dfclty)
//		dt := time.Since(tt)

		if err != nil {
			errnum++
		}

		if err == nil && (addr != daddr || ec != dec) {
			errpositive++

			fmt.Printf("! addr %d ec %v {", addr, ec)
			for _, v := range blks {
				fmt.Printf("%d, ", v)
			}
			fmt.Printf("}\n")
			fmt.Printf("# daddr %d dec %v {", daddr, dec)
			for _, v := range dblks {
				fmt.Printf("%d, ", v)
			}
			fmt.Printf("}\n")

			fmt.Printf("- %v\n", ol)
			fmt.Printf("+ %v\n", eol)

		}
/*
		if err != nil {
			t.Fatalf("error while decoding: %v\n", err)
		}

		if addr != daddr {
			t.Fatalf("addresses don't match: %v %v\n", addr, daddr)
		}

		if ec != dec {
			t.Fatalf("erasure flag doesn't match: %v %v\n", ec, dec)
		}
*/

//		fmt.Printf("orig: %v\nerr:  %v %v %v\n", ol, eol, et, dt)

	}

//	fmt.Printf("error rate %v false positive rate %v\n", float64(errnum)/float64(niter), float64(errpositive)/float64(niter))
	fmt.Printf("%d %d %d %s %d %v %v\n", *dbnum, *mdnum, *mdcnum, *mdctype, *dfclty, float64(errnum)/float64(niter), float64(errpositive)/float64(niter))
}
