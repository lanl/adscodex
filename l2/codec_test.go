package l2

import (
	"flag"
	"fmt"
	"os"
	"math/rand"
	"testing"
//	"time"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/oligo/short"
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
	cdc = NewCodec(p5, p3, 5, 4, 2, 3, 2)
}

func TestEncode(t *testing.T) {
	initTest(t)

	fmt.Printf("TestEncode:\n")
	for n := 0; n < 2; n++ {
		sz := rand.Intn(1<<16)
		data := make([]byte, sz)
		for i := 0; i < sz; i++ {
			data[i] = byte(rand.Intn(256))
		}

		fmt.Printf("--------------- %d bytes ------------\n", sz)
		nextaddr, oligos, err := cdc.Encode(0, data)
		if err != nil {
			t.Fatalf("encode error: %v\n", err)
		}

		fmt.Printf("\nDecoding...\n")
		de := cdc.Decode(0, nextaddr, oligos)
		if len(de) == 0 {
			t.Fatalf("decode error no data\n")
		} else if len(de) != 1 {
			t.Fatalf("decode error: too many data extents: %d\n", len(de))
		}

		ddata := de[0].Data
		if len(data) != len(ddata) {
			t.Fatalf("decoded data length doesn't match: %d %d\n", len(data), len(ddata))
		}

		for i := 0; i < len(data); i++ {
			if data[i] != ddata[i] {
				fmt.Printf("%v\n", data)
				fmt.Printf("%v\n", ddata)
				t.Fatalf("decoded data doesn't match")
			}
		}
	}
}

func TestErrors(t *testing.T) {
	initTest(t)

	fmt.Printf("TestErrors:\n")
	for n := 0; n < 2; n++ {
		fmt.Printf("Try %d\n", n)
		sz := rand.Intn(1<<16)
		data := make([]byte, sz)
		for i := 0; i < sz; i++ {
			data[i] = byte(rand.Intn(256))
		}

		fmt.Printf("--------------- %d bytes ------------\n", sz)
		nextaddr, oligos, err := cdc.Encode(0, data)
		if err != nil {
			t.Fatalf("encode error: %v\n", err)
		}

		// add another copy of the first oligo
		oligos = append(oligos, oligos[0])

		// add an oligo with an error
		o := oligos[rand.Int31n(int32(len(oligos) - 1))]
		p := int(rand.Int31n(int32(o.Len())))
		o1 := o.Slice(0, p)
		o1.Append(short.FromString1("T"))
		o1.Append(o.Slice(p+1, 0))
		oligos = append(oligos, o1)

		fmt.Printf("\nDecoding...\n")
		de := cdc.Decode(0, nextaddr, oligos)
		if len(de) == 0 {
			t.Fatalf("decode error no data\n")
		} else if len(de) != 1 {
			t.Fatalf("decode error: too many data extents: %d\n", len(de))
		}

		ddata := de[0].Data
		if len(data) != len(ddata) {
			t.Fatalf("decoded data length doesn't match: %d %d\n", len(data), len(ddata))
		}

		for i := 0; i < len(data); i++ {
			if data[i] != ddata[i] {
				fmt.Printf("%v\n", data)
				fmt.Printf("%v\n", ddata)
				t.Fatalf("decoded data doesn't match")
			}
		}
	}
}
