package l0

import (
	"container/heap"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"adscodex/criteria"
	"adscodex/errmdl"
_	"adscodex/oligo"
	"adscodex/oligo/short"
)

type GenTable struct {
	olen	int
	entries	[]GenEntry
	heap	[]int		// heap of the oligos sorted by (lowest) probability of error
	etbl	*GenErrorTable
}

type GenEntry struct {
	prob	float64
	sources	[]GenSource
}

type GenSource struct {
	nol	uint64
	eol	*short.Oligo
	prob	float64
}

type GenErrorTable struct {
	olen	int
	em	errmdl.ErrMdl
	errs	[]GenErrOligo
}

type GenErrOligo struct {
	err	float32			// total errors
	errmin	float32			// the highest error from all oligos
	errs	[]errmdl.OligoProb	// errors as reported by the error model
	cerrs	[]errmdl.OligoProb	// errors with the oligos the same length as the original oligo
}

func BuildLookupTable(oligoLen, maxVal, pfxLen int, minerr float64, crit criteria.Criteria, emdl errmdl.ErrMdl) *LookupTable {
	lt := newLookupTable(oligoLen, maxVal, pfxLen, minerr, crit, emdl)

	npfx := 1<<(2*pfxLen)
	ncpu := 1 // runtime.NumCPU()
	sperproc := 1 + npfx/ncpu
	pn := 0
	ch := make(chan bool)
	for i := 0; i < ncpu; i++ {
		start := i*sperproc
		end := (i+1) * sperproc
		if start >= npfx {
			break
		}

		if end > npfx {
			end = npfx
		}

		go func() {
			for np := uint64(start); np < uint64(end); np++ {
				lt.generateTable(np)
			}

			ch <- true
		}()
		pn++
	}

	// wait until all goroutines are done
	for i := 0; i < pn; i++ {
		<-ch
	}


//	for npfx := uint64(0); npfx < (1<<(2*pfxLen)); npfx++ {
//		lt.generateTable(npfx)
//	}

	return lt
}

func (lt *LookupTable) generateTable(npfx uint64) {
	pfx := short.Val(lt.pfxLen, npfx)

	fmt.Printf("generate '%v'\n", pfx)
	etbl := newErrorTable(lt.oligoLen, lt.emdl, lt.minerr)
	tbl := newGenTable(lt.oligoLen, etbl)
	count := lt.maxVal

	// first mark all invalid oligos with error probability of 10
	for n := uint64(0); n < uint64(len(tbl.entries)); n++ {
		ol := short.Val(tbl.olen, n);
		pol := pfx.Clone()
		pol.Append(ol)
		if !lt.crit.Check(pol) {
			tbl.entries[n].prob = 10
		} else {
			tbl.entries[n].prob = 0
		}
	}

	// pick the best sequences
	nols := make(map[uint64]bool)
	for {
		// pick the best oligo
		tbl.FixHeap()
		nol := uint64(tbl.heap[0])
		tbl.heap = tbl.heap[1:]

		nols[nol] = true

		errs := etbl.errs[nol].errs
		for i := 0; i < len(errs); i++ {
			e := &errs[i]
			canonize(e, etbl.olen, func(enol uint64, prob float64) {
				en := &tbl.entries[enol]
				en.sources = append(en.sources, GenSource{nol, e.Ol.(*short.Oligo), prob})
				if enol != nol {
					en.prob += prob
				}
			})
		}

		if len(nols) == count {
			break
		}
	}

	// create an array with the sequences
	var ols []int
	for k, _ := range nols {
		ols = append(ols, int(k))
	}
	sort.Ints(ols)

	// subtract the unrealistically high value from the bad sequences
	for n := uint64(0); n < uint64(len(tbl.entries)); n++ {
		ol := short.Val(tbl.olen, n);
		if !lt.crit.Check(ol) {
			tbl.entries[n].prob -= 10
		}
	}


	// allocate and fill the encoding table
	olmap := make(map[uint64]int)
	ectbl := new(EncTable)
	ectbl.oligos = make([]short.Oligo, len(ols))
	lt.etbls[npfx] = ectbl
	for i := 0; i < len(ols); i++ {
		ectbl.oligos[i].SetVal(lt.oligoLen, uint64(ols[i]))
		olmap[uint64(ols[i])] = i
	}

	// allocate and fill the decoding table
	// iterate through all the oligos and print what they are closest to
	olen := lt.oligoLen
	alen := 1<<(2*olen)

	dctbl := new(DecTable)
	dctbl.entries = make([][VariantNum]DecVariant, alen)
	lt.dtbls[npfx] = dctbl
	for nol := uint64(0); nol < uint64(len(tbl.entries)); nol++ {
		en := &tbl.entries[nol]
		dent := &dctbl.entries[nol]
		sort.Slice(en.sources, func(i, j int) bool {
			return en.sources[i].prob > en.sources[j].prob
		})

		// invalid entry -> olen = 0
		for i := 0; i < VariantNum; i++ {
			dent[i].ol.SetVal(0, 0)
		}

		for i := 0; i < len(en.sources) && i < VariantNum; i++ {
			s := &en.sources[i]
			dent[i].val = uint16(olmap[s.nol])
			dent[i].ol.SetVal(s.eol.Len(), s.eol.Uint64())
			dent[i].prob = float32(s.prob)
		}
	}
}

func canonize(olp *errmdl.OligoProb, olen int, fn func(nol uint64, prob float64)) {
	olplen := olp.Ol.Len()
	if olplen == olen {
		nol := olp.Ol.(*short.Oligo).Uint64()
		fn(nol, olp.Prob)
	} else if olplen > olen {
		// the oligo is longer, just cut it
		nol := olp.Ol.Slice(0, olen).(*short.Oligo).Uint64()
		fn(nol, olp.Prob) 
	} else {
		// the oligo is shorter, have to iterate through all and add portion of the probability
		shift := 2 * (olen - olp.Ol.Len())
		n := 1<<shift
		prob := olp.Prob / float64(n)
		snol := olp.Ol.(*short.Oligo).Uint64() << shift;
		for i := 0; i < n; i++ {
			nol := snol + uint64(i)
			fn(nol, prob)
		}
	}
}

func newErrorTable(olen int, em errmdl.ErrMdl, minerr float64) *GenErrorTable {
	alen := 1 << (2 * olen)

	etbl := new(GenErrorTable)
	etbl.olen = olen
	etbl.em = em
	etbl.errs = make([]GenErrOligo, alen)

	ncpu := runtime.NumCPU()
	sperproc := 1 + alen/ncpu
	pn := 0
	ch := make(chan bool)
	for i := 0; i < ncpu; i++ {
		start := i*sperproc
		end := (i+1) * sperproc
		if start >= alen {
			break
		}

		if end > alen {
			end = alen
		}

		go func() {
			for nol := uint64(start); nol < uint64(end); nol++ {
				eol := &etbl.errs[nol]

				ol := short.Val(etbl.olen, nol);
				eol.errs = em.SortedErrors(ol, minerr)
				cmap := make(map[uint64]float64)
				for i := 0; i < len(eol.errs); i++ {
					e := &eol.errs[i]
					canonize(e, etbl.olen, func(enol uint64, prob float64) {
						cmap[enol] += prob
					})

					enol := e.Ol.(*short.Oligo).Uint64()
					if nol == enol {
						// skip itself
						continue
					}

					eol.err += float32(e.Prob)
					if float32(e.Prob) > eol.errmin {
						eol.errmin = float32(e.Prob)
					}
				}

				// prepare the list of canonical errors
				eol.cerrs = make([]errmdl.OligoProb, len(cmap))
				i := 0
				for ol, prob := range cmap {
					e := &eol.cerrs[i]
					e.Ol = short.Val(etbl.olen, ol)
					e.Prob = prob
					i++
				}

				sort.Slice(eol.cerrs, func(i, j int) bool {
					return eol.cerrs[i].Prob > eol.cerrs[j].Prob
				})
			}

			ch <- true
		}()

		pn++
	}

	// wait until all goroutines are done
	for i := 0; i < pn; i++ {
		<-ch
	}

	return etbl
}

func newGenTable(olen int, etbl *GenErrorTable) *GenTable {
	tbl := new(GenTable)
	tbl.olen = olen
	tbl.etbl = etbl

	alen := 1 << (2 * olen)
	tbl.entries = make([]GenEntry, alen)
	tbl.heap = make([]int, alen)

	for n := 0; n < alen; n++ {
		tbl.heap[n] = n
	}

	rand.Shuffle(len(tbl.heap), func (i, j int) {
		tbl.heap[i], tbl.heap[j] = tbl.heap[j], tbl.heap[i]
	})

	return tbl
}

func (t *GenTable) Less(i, j int) bool {
//	fmt.Printf("Less %d %d\n", i, j)
	ii := t.heap[i]
	jj := t.heap[j]

	pi := t.entries[ii].prob
	pj := t.entries[jj].prob

	if pi == pj {
		return t.etbl.errs[ii].err < t.etbl.errs[jj].err
	}

	return pi < pj
}

func (t *GenTable) Swap(i, j int) {
//	fmt.Printf("Swap %d %d\n", i, j)
	t.heap[i], t.heap[j] = t.heap[j], t.heap[i]
}

func (t *GenTable) FixHeap() {
	heap.Init(t)
}

func (t *GenTable) Len() int {
	return len(t.heap)
}

func (t *GenTable) Pop() (ret interface{}) {
	ret = t.heap[0]
	t.heap = t.heap[1:]
	return
}

func (t *GenTable) Push(x interface{}) {
	fmt.Printf("push\n")
	return
}
