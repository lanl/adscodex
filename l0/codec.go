package l0

// Functions for reading and writing lookup tables
// TODO: describe the on-disk format
import (
	"fmt"
_	"math"
	"math/rand"
	"sync"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
_	"adscodex/criteria"
)

type Group struct {
	sync.Mutex
	lts		[]*LookupTable
	olen		int
	prefix		oligo.Oligo
	maxtime		int64
	triecat		*Trie
	rnd		*rand.Rand
}

// Concatenate the back trie to all of the front ones depending on their prefixes.
// The index in the array is the short.Val representation of the prefix.
// The assumption is that the length of the oligos in the trie is bigger
// or equal to the prefix length.
func concatTries(from, back []*Trie, pfxlen int) {
	for p, t := range from {
		t.Concat(pfxlen, uint64(p), back)
	}
}

// maxtime is in ms
func NewGroup(prefix oligo.Oligo, lts []*LookupTable, maxtime int64) (g *Group, err error) {
	g = new(Group)
	g.lts = lts
	g.rnd = rand.New(rand.NewSource(time.Now().UnixMilli()))
	g.maxtime = maxtime

	if prefix.Len() < g.lts[0].pfxlen {
		err = fmt.Errorf("prefix too short")
		return
	}
	g.prefix = prefix.Slice(prefix.Len() - g.lts[0].pfxlen, 0)

	// concatenate the tries for each field, starting from the back
	var pts []*Trie
	for i := len(g.lts) - 1; i >= 0; i-- {
//		fmt.Printf("Concat %d\n", i)
		lt := g.lts[i]
		g.olen += lt.oligolen

		// clone the tries so we don't affect the originals
		cts := make([]*Trie, len(lt.pfxtbl))
		for j := 0; j < len(cts); j++ {
			if lt.pfxtbl[j] != nil {
				cts[j] = lt.pfxtbl[j].trie.Clone()
			}
		}

		if pts != nil {
			concatTries(cts, pts, lt.pfxlen)
		}

		pts = cts
	}

	// concatenate the prefix
	pfx, ok := short.Copy(prefix.Slice(prefix.Len() - g.lts[0].pfxlen, 0))
	if !ok {
		err = fmt.Errorf("prefix too long")
		return
	}

	g.triecat, _ = NewTrie(nil)
	g.triecat.add(pfx, 0)
	g.olen += pfx.Len()
//	fmt.Printf("Concat prefix %d\n", pfx.Len())
	g.triecat.Concat(pfx.Len(), 0, pts)

//	fmt.Printf("group length %d triecat size %d\n", g.olen, g.triecat.Size())
	return
}

func (g *Group) match(ol oligo.Oligo) (ret oligo.Oligo, dist int) {

	// create random order of bps per position
	bporder := make([]int, g.olen * 4)
	for i := 0; i < len(bporder); i++ {
		bporder[i] = i%4
	}

	g.Lock()
	for i := 0; i < g.olen; i++ {
		n := i*4
		g.rnd.Shuffle(4, func(i, j int) {
			bporder[n+i], bporder[n+j] = bporder[n+j], bporder[n+i]
		})
	}
	g.Unlock()

	stoptime := int64(-1)
	if g.maxtime > 0 {
		stoptime = time.Now().Add(time.Duration(g.maxtime) * time.Millisecond).UnixMilli()
	}

	m := g.triecat.SearchMin(ol, bporder, stoptime)
	if m != nil {
		ret = m.Seq
		dist = m.Dist
	}

	return
}

func (g *Group) Encode(vals []int) (ret oligo.Oligo, err error) {
	if len(vals) != len(g.lts) {
		err = fmt.Errorf("vals number mismatch expected %d got %d\n", len(g.lts), len(vals))
		return
	}

	ret, _ = long.Copy(g.prefix.Clone())
	for i, lt := range g.lts {
		if uint64(vals[i]) >= lt.maxval {
			err = fmt.Errorf("value for field %d above the limit %d:%d\n", i, vals[i], lt.maxval)
			return
		}

		pfx, ok := short.Copy(ret.Slice(ret.Len() - lt.pfxlen, 0))
		if !ok {
			panic("can't happen")
		}

		pidx := pfx.Uint64()
		fol := short.Val(lt.oligolen, lt.pfxtbl[pidx].etbl[vals[i]])
		if !ret.Append(fol) {
			panic("shouldn't happen")
		}

//		fmt.Printf("%d: pfx %v val %d fol %v ret %v\n", i, pfx, vals[i], fol, ret)
	}

	ret = ret.Slice(g.prefix.Len(), 0)
	return
}

func (g *Group) Decode(prefix, ol oligo.Oligo) (vals []int, dist int, err error) {
	if ol == nil {
		panic("ol is nil")
	}

	if prefix.Len() < g.lts[0].pfxlen {
		err = fmt.Errorf("prefix too short")
		return
	}

	seq, _ := long.Copy(prefix.Slice(prefix.Len() - g.lts[0].pfxlen, 0))
	seq.Append(ol)

	match, d := g.match(seq)
	if match == nil {
		err = fmt.Errorf("no match")
		return
	}

	if match.Len() != g.olen {
		panic("shouldn't happen")
	}

//	fmt.Printf("match %v\n", match)
	dist = d
	vals = make([]int, len(g.lts))
	pos := g.lts[0].pfxlen
	for i, lt := range g.lts {
		pfx := match.Slice(pos - lt.pfxlen, pos)
		fld := match.Slice(pos, pos+lt.oligolen)
		pos += lt.oligolen

		vals[i] = int(lt.decodeLookup(pfx, fld))
	}

	return
}
