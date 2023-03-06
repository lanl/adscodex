package l0

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
	"adscodex/criteria"
)

//var iternum = flag.Int("n", 5, "number of iterations")
var tblname = flag.String("tbl", "", "lookup table")
var olen = flag.Int("olen", 4, "if nonzero, generate lookup tables for the oligo length")
//var ebits = flag.Int("eb", 1, "bits for the encoding lookup tables (if n is set)")
//var dbits = flag.Int("db", 1, "bits for the encoding lookup tables (if n is set)")
var mindist = flag.Int("mindist", 3, "minimum distance")

var ltable *LookupTable

func randomString(l int) string {
	// don't allow oligos of 0 length
	if l == 0 {
		l = 1
	}

	so := ""
	for i := 0; i < l; i++ {
		so += oligo.Nt2String(rand.Intn(4))
	}

	return so
}

func randomOligo(l int) oligo.Oligo {
	so := randomString(l)

	// randomly return some of the oligos as short, so we can test
	// the interoperability
	if l < 31 && rand.Intn(2) == 0 {
		return short.FromString1(so)
	}
		
	return long.FromString1(so)
}

func loadLookupTables() {
	lt, err := LoadLookupTable("../tbl/o10m3.tbl")
	if err != nil {
		fmt.Printf("error while loading lookup table: %v\n", err)
	}

	ltable = lt
}

func TestMain(m *testing.M) {
	flag.Parse()
	loadLookupTables()
	os.Exit(m.Run())
}

func TestGen(t *testing.T) {
	if *olen == 0 {
		return
	}

	crit := criteria.H4G2
//	prefix, _ := short.FromString("AAAA")
//	tbl := BuildTable(prefix, nil, 10, 4, crit)
	lt := BuildLookupTable(crit, *olen, *mindist, true, 0)
	fmt.Printf("maxval %d\n", lt.maxval)
}

func TestIO(t *testing.T) {
	if *tblname == "" {
		return
	}

	lt, err := LoadLookupTable(*tblname)
	if err != nil {
		t.Fatalf("error while loading lookup table: %v\n", err)
	}

	fmt.Printf("table %s maxval %d\n", *tblname, lt.maxval)
}

/*
func TestPrint(t *testing.T) {
	return

	prefix := short.FromString1("CTAA")
	fmt.Printf("loading table\n")
	err := LoadEncodeTable("../tbl/encnt17b20.tbl", criteria.H4G2)
	if err != nil {
		t.Fatalf("error while loading encoding table: %v\n", err)
	}

	lt := encodeTables[criteria.H4G2][17]
	tbl := lt.pfxtbl[prefix.Uint64()]
	fmt.Printf("%v\n", tbl.String(lt.oligolen))
}

func TestEncodeTable(t *testing.T) {
	return

	tbl := BuildEncodingTable(short.FromString1("CTAA"), 17, 20, criteria.H4G2)
	fmt.Printf("\tbits %d prefix %v maxval %d\n", tbl.bits, tbl.prefix, tbl.maxval)
	for i, v := range tbl.tbl {
		fmt.Printf("\t%d: %v\n", i, short.Val(4, v))
	}

	return
	lt := BuildEncodingLookupTable(4, 4, 0, criteria.H4G2)
	if err := lt.Write("t"); err != nil {
		t.Fatalf("Error while writing: %v\n", err)
	}

	if _, err := readLookupTable("t", criteria.H4G2); err != nil {
		t.Fatalf("Error while reading: %v\n", err)
	} else {

		fmt.Printf("oligolen %d pfxlen %d pfxtbl %d\n", lt.oligolen, lt.pfxlen, len(lt.pfxtbl))
		for p := 0; p < len(lt.pfxtbl); p++ {
			fmt.Printf("Table for %v\n", short.Val(4, uint64(p)))
			tbl := lt.pfxtbl[p]
			if tbl == nil {
				continue
			}
			fmt.Printf("\tbits %d prefix %v maxval %d\n", tbl.bits, tbl.prefix, tbl.maxval)
			for i, v := range tbl.tbl {
				fmt.Printf("\t%d: %v\n", i, short.Val(lt.oligolen, v))
			}
		}
	}
}

func TestDecodeTable(t *testing.T) {
	return

	tbl := BuildDecodingTable(short.FromString1("AAAA"), 4, 0, criteria.H4G2)
	fmt.Printf("\tbits %d prefix %v maxval %d\n", tbl.bits, tbl.prefix, tbl.maxval)
	for i, v := range tbl.tbl {
		fmt.Printf("\t%v: %v\n", short.Val(4, uint64(i)), v)
	}

	return

	lt := BuildDecodingLookupTable(4, 4, 1, criteria.H4G2)
	if err := lt.Write("t"); err != nil {
		t.Fatalf("Error while writing: %v\n", err)
	}

	if _, err := readLookupTable("t", criteria.H4G2); err != nil {
		t.Fatalf("Error while reading: %v\n", err)
	} else {

	}
}
*/
