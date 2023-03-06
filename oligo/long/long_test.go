package long

import (
	"flag"
	"math/rand"
	"os"
	"testing"
	"adscodex/oligo"
	"adscodex/oligo/short"
)

var iternum = flag.Int("n", 5, "number of iterations")

func TestMain(m *testing.M) {
	flag.Parse()
        os.Exit(m.Run())
}

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

func randomOligo(l int) (o oligo.Oligo) {
	so := randomString(l)

	// randomly return some of the oligos as short, so we can test
	// the interoperability
	if l < 31 && rand.Intn(3) == 0 {
		o, _ = short.FromString(so)
	} else {
		o, _ = FromString(so)
	}

	return
}

func TestAt(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		so1 := randomString(rand.Intn(47))
		o1, _ := FromString(so1)
		so2 := ""
		for i := 0; i < o1.Len(); i++ {
			so2 += oligo.Nt2String(o1.At(i))
		}

		if so1 != so2 {
			t.Fatalf("At() fails: %v: %v", so1, so2)
		}
	}
}

func TestString(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		so1 := randomString(rand.Intn(47))
		o1, _ := FromString(so1)
		so2 := o1.String()

		if so1 != so2 {
			t.Fatalf("String() fails: %v: %v", so1, so2)
		}
	}
}

func TestCmp(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		o1 := randomOligo(rand.Intn(47))
		o2 := randomOligo(rand.Intn(47))

		so1 := o1.String()
		so2 := o2.String()

		scmp := len(so1) - len(so2)
		if scmp < 0 {
			scmp = -1
		} else if scmp > 0 {
			scmp = 1
		}

		if scmp == 0 {
			for i := 0; i < len(so1); i++ {
				nt1 := oligo.String2Nt(string(so1[i]))
				nt2 := oligo.String2Nt(string(so2[i]))
				if nt1 < nt2 {
					scmp = -1
					break
				} else if nt1 > nt2 {
					scmp = 1
					break
				}
			}
		}

		if scmp != o1.Cmp(o2) {
			t.Fatalf("Cmp() fails: %v:%v %d:%d", so1, so2, scmp, o1.Cmp(o2))
		}
	}
}

func TestNext(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		o1 := randomOligo(rand.Intn(47))
		o2 := o1.Clone()
		o2.Next()

		if o1.Cmp(o2) != -1 {
			t.Fatalf("Next() fails: %v: %v", o1, o2)
		}
	}
}


func TestSlice(t *testing.T) {
	for i := 0; i < *iternum; {
		o1 := randomOligo(rand.Intn(47))
		if o1.Len() < 4 {
			continue
		}

		s := rand.Intn(o1.Len())
		e := rand.Intn(o1.Len())

		if (e <= s) {
			continue
		}

		so1 := ""
		for n := s; n < e; n++ {
			so1 += oligo.Nt2String(o1.At(n))
		}

		o2 := o1.Slice(s, e)
		so2 := o2.String()
		if so1 != so2 {
			t.Fatalf("Slice() fails: %v: %v", so1, so2)
		}

		i++
	}
}

func TestAppend(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		o1 := randomOligo(rand.Intn(47))
		o2 := randomOligo(rand.Intn(47))

		so1 := o1.String() + o2.String()
		ok := o1.Append(o2)
		so2 := o1.String()

		if !ok || so1 != so2 {
			t.Fatalf("Append() fails: %v: %v", so1, so2)
		}
	}
}

func TestZeroAppend(t *testing.T) {
	o1 := New(0)
	o2 := randomOligo(rand.Intn(47))

	o1.Append(o2)
	if o1.Cmp(o2) != 0 {
		t.Fatalf("append to empty oligo")
	}
}

func TestCopy(t *testing.T) {
	for i := 0; i < *iternum; i++ {
		o1 := randomOligo(rand.Intn(47))
		o2, ok := Copy(o1)

		if !ok || o1.Cmp(o2) != 0 {
			t.Fatalf("Copy() fails: %v: %v", o1, o2)
		}
	}
}

func TestFind(t *testing.T) {
	seq, _ := FromString("CGAACATCTCGATGGCAGCATCACCGTTCGCCGAAGCAAAATACCCCTTCCG")
	sub, _ := FromString("CGACATCTCGATGGC")

	p, l := oligo.Find(seq, sub, 1)
	if p != 0 || l != 16 {
		t.Fatalf("Find %v in %v should return 0:16 instead of %d:%d", sub, seq, p, l)
	}
}

func TestFind2(t *testing.T) {
	seq, _ := FromString("CGGTATTTAGTGAATAC")
	sub, _ := FromString("GGCGGTATTT")

	p, l := oligo.Find(seq, sub, 5)
	if p != 0 || l != 8 {
		t.Fatalf("Find %v in %v should return 0:16 instead of %d:%d", sub, seq, p, l)
	}
}

func TestDist(t *testing.T) {
	o1, _ := FromString("CGACATCTCGATGGCAGCATCACCGTTCGCCGAAGCAAAATACCCCTTCCG")
	o2, _ := FromString("CGACACCTCGATGGCAGCATCACCGTTCGCCGAACAAAATACCCCTTCCG")

	d := oligo.Distance(o1, o2)
	if d != 2 {
		t.Fatalf("Distance %v:%v should be 2 instead of %d", o1, o2, d)
	}

	o1, _ = FromString("CGACATCTCGATGGCAGCATATGCAGACTAGATCAACAAAATTAGACCAGCTAACTCGAGCTTATCAGTTTGTACTTATTTTGAAATGTACAGCGTAGGAATGATTATACGACAAGTACATAAAGCCCAGTGAGCTGGCAACTTCCA")
	o2, _ = FromString("CGACATCTCGATGGCAGCATATGCAGACTAGATCAACAAAATTAGACCAGCTAACTCGAGCTTATCAGTTTGTACTTATTTTGAAATGTACAGCGTAGGAATGATTATACGACAAGTACATAAAGTCCAGTGAGCTGGCAACTTCCA")
	dist, diff := oligo.Diff(o1, o2)
	if dist != 1 {
		t.Fatalf("Distance %v:%v should be 1 instead of %d", o1, o2, d)
	}

	if diff != "-----------------------------------------------------------------------------------------------------------------------------R---------------------" {
		t.Fatalf("Diff %v:%v should be '-----------------------------------------------------------------------------------------------------------------------------R---------------------' instead of %v", o1, o2, diff)
	}
}
