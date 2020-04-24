package l1

import (
	"flag"
	"fmt"
	"os"
	"math/rand"
	"testing"
	"time"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
	"acoma/criteria"
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

var cdc *Codec
var p5, p3 oligo.Oligo

func initTest(t *testing.T) {
	if cdc != nil {
		return
	}

	err := l0.LoadEncodeTable("../tbl/encnt17b13.tbl", criteria.H4G2)
	if err != nil {
		t.Fatalf("error while loading encoding table: %v\n", err)
	}

	err = l0.LoadDecodeTable("../tbl/decnt17b7.tbl", criteria.H4G2)
	if err != nil {
		t.Fatalf("error while loading decoding table: %v\n", err)
	}

	p5, _ = long.FromString("CGACATCTCGATGGCAGCA")
	p3, _ = long.FromString("ATCAGTGAGCTGGCAACTTCCA")

	cdc = NewCodec(5, 4, 2, criteria.H4G2)
}

func TestEncode(t *testing.T) {
	initTest(t)

	maxaddr := cdc.MaxAddr()
	fmt.Printf("maxaddr: %d\n", maxaddr)

	blks := make([][]byte, cdc.BlockNum())
	for i := 0; i < len(blks); i++ {
		blks[i] = make([]byte, cdc.BlockSize())
	}

	for n := 0; n < 200; n++ {
		for i := 0; i < len(blks); i++ {
			for j := 0; j < len(blks[i]); j++ {
				blks[i][j] = byte(rand.Intn(256))
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

		tt = time.Now()
		daddr, dec, dblks, err := cdc.Decode(p5, p3, ol, false)
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
			if dblks[i] == nil {
				t.Fatalf("nil data block")
			}

			if len(blks[i]) != len(dblks[i]) {
				t.Fatalf("blocks length differ")
			}

			for j := 0; j < len(blks[i]); j++ {
				if blks[i][j] != dblks[i][j] {
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

	// CGACATCTCGATGGCAGCA TGAGCTCAAACTCATGT TACG TCCTTTTGAGTTAACAA ATTCG ATCAGTGAGCTGGCAACTTCCA
	//                     TGAGCTCAAACTCATGTTACGTCCTTTTGAGTTAACAAATTCG
	// addr 28 ec true dblks [[33 15 199 187] [129 134 57 172]]

	cdc := NewCodec(2, 4, 1, criteria.H4G2)

	// one insert in the first data block
	ol, _ := long.FromString("CGACATCTCGATGGCAGCATGGAGCTCAAACTCATGTTACGTCCTTTTGAGTTAACAAATTCGATCAGTGAGCTGGCAACTTCCA")
	daddr, dec, _, err := cdc.Decode(p5, p3, ol, true)
	if err != nil {
		t.Fatalf("Error while recovering %v: %v\n", ol, err)
	} else if daddr != 28 || !dec {
		t.Fatalf("Error metadata incorrect (%v, %v) expected (28, true)\n", daddr, dec)
	}

//	fmt.Printf("TestRecover insert first %d %v %v %v\n", daddr, dec, dblks, err)

	// one delete in the first data block
	ol, _ = long.FromString("CGACATCTCGATGGCAGCATGAGCTCAACTCATGTTACGTCCTTTTGAGTTAACAAATTCGATCAGTGAGCTGGCAACTTCCA")
	daddr, dec, _, err = cdc.Decode(p5, p3, ol, true)
	if err != nil {
		t.Fatalf("Error while recovering %v: %v\n", ol, err)
	} else if daddr != 28 || !dec {
		t.Fatalf("Error metadata incorrect (%v, %v) expected (28, true)\n", daddr, dec)
	}
//	fmt.Printf("TestRecover delete first %d %v %v %v\n", daddr, dec, dblks, err)

	// one insert in the second data block
	ol, _ = long.FromString("CGACATCTCGATGGCAGCATGAGCTCAAACTCATGTTACGTCCTTTTGGAGTTAACAAATTCGATCAGTGAGCTGGCAACTTCCA")
	daddr, dec, _, err = cdc.Decode(p5, p3, ol, true)
	if err != nil {
		t.Fatalf("Error while recovering %v: %v\n", ol, err)
	} else if daddr != 28 || !dec {
		t.Fatalf("Error metadata incorrect (%v, %v) expected (28, true)\n", daddr, dec)
	}
//	fmt.Printf("TestRecover insert second %d %v %v %v\n", daddr, dec, dblks, err)

	// one delete in the second data block
	ol, _ = long.FromString("CGACATCTCGATGGCAGCATGAGCTCAAACTCATGTTACGTCCTTTTGAGTAACAAATTCGATCAGTGAGCTGGCAACTTCCA")
	daddr, dec, _, err = cdc.Decode(p5, p3, ol, true)
	if err != nil {
		t.Fatalf("Error while recovering %v: %v\n", ol, err)
	} else if daddr != 28 || !dec {
		t.Fatalf("Error metadata incorrect (%v, %v) expected (28, true)\n", daddr, dec)
	}
//	fmt.Printf("TestRecover delete second %d %v %v %v\n", daddr, dec, dblks, err)
}

