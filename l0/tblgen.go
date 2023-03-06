package l0

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"
	"adscodex/criteria"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
)

// Build an encoding table for the specified prefix, oligo length and criteria
// Right now this all works for short oligos only
func BuildTable(prefix *short.Oligo, startol oligo.Oligo, olen, mindist int, c criteria.Criteria, shuffle bool, maxval int) (tbl *Table) {
	tbl = newTable(prefix, olen, mindist)

	if startol == nil {
		startol = short.New(olen)
	}

	ol := startol.Clone()
	for {
		var nol oligo.Oligo
		if prefix != nil {
			nol = prefix.Clone()
			nol.Append(ol)
		} else {
			nol = ol
		}

		if gc := oligo.GCcontent(ol); c.Check(nol) && gc > 0.4 && gc < 0.6 {
			m := tbl.trie.SearchMin(ol, nil, -1)
			if m == nil || m.Dist >= tbl.mindist {
				tbl.trie.Add(ol, 0)
				shortol, ok := short.Copy(ol)
				if !ok {
					panic("oligo too big")
				}

				tbl.etbl = append(tbl.etbl, shortol.Uint64())
			}
		}

		if !ol.Next() {
//			fmt.Printf("no next for %v\n", ol)
			ol = long.New(olen)
		}

//		fmt.Printf("next %v:%v %v\n", ol, startol, ol.Cmp(startol))
		if ol.Cmp(startol) == 0 {
			break
		}
	}

	if shuffle {
		rand.Seed(11)
		rand.Shuffle(len(tbl.etbl), func (i, j int) { tbl.etbl[i], tbl.etbl[j] = tbl.etbl[j], tbl.etbl[i] })
	}

	if maxval > 0 && len(tbl.etbl) > maxval {
		tbl.etbl = tbl.etbl[0:maxval]
	}

	tbl.maxval = uint64(len(tbl.etbl))

	// FIXME: should we shuffle the oligos so the consequent values are not close oligos???
	for v, o := range tbl.etbl {
		tbl.dmap[o] = v
	}

	if tbl.maxval == 0 {
		return nil
	}

	return
}

// build encoding tables for all different prefixes
func BuildLookupTable(c criteria.Criteria, olen, mindist int, shuffle bool, maxval int) (ltbl *LookupTable) {
	ltbl = new(LookupTable)
	ltbl.oligolen = olen
	ltbl.pfxlen = c.FeatureLength()
	ltbl.mindist = mindist
	ltbl.crit = c

	prefix := (*short.Oligo)(short.New(ltbl.pfxlen))
	ltbl.pfxtbl = make([]*Table, (1<<(ltbl.pfxlen*2)))
	pfxch := make(chan *short.Oligo)
	done := make(chan bool)
	procnum := runtime.NumCPU()
	for i := 0; i < procnum; i++ {
		go func() {
			for {
				pfx := <-pfxch
				if pfx == nil {
					done <- true
					return
				}

				fmt.Printf("%v start %v\n", pfx, time.Now())
				idx := pfx.Uint64()
				ltbl.pfxtbl[idx] = BuildTable(pfx, nil, olen, mindist, c, shuffle, maxval)
				fmt.Printf("%v completed\n", pfx)
			}
		}()
	}

	n := 0
	for n < procnum {
		select {
		case <- done:
			n++

		case pfxch <- prefix:
			if prefix != nil {
				prefix = prefix.Clone().(*short.Oligo)
				if !prefix.Next() {
					prefix = nil
				}
			}
		}
	}

	ltbl.maxval = ltbl.pfxtbl[0].maxval	
	for _, tbl := range ltbl.pfxtbl {
		if tbl != nil && ltbl.maxval < tbl.maxval {
			ltbl.maxval = tbl.maxval
		}
	}

	return ltbl
}
