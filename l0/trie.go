package l0

import (
_	"fmt"
_	"errors"
	"math"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
_	"adscodex/oligo/short"
)

// how many steps searchMinRecursive does before checking if it should stop
const CheckTimeCount = 50000

type Trie struct {
	bp		byte			// current base pair
	depth		int			// how many levels under it
	strand		oligo.Oligo
	chld		[4]*Trie		// children
}

type DistSeq struct {
	Seq	oligo.Oligo
	Dist	int
}

func NewTrie(seqs []oligo.Oligo) (trie *Trie, err error) {
	trie = new(Trie)
	trie.bp = math.MaxUint8
	for _, seq := range seqs {
		err = trie.Add(seq, 0)
		if err != nil {
			trie = nil
			return
		}
	}

	return
}

func (t *Trie) add(strand oligo.Oligo, idx int) (depth int, err error) {
	if idx == strand.Len() {
		t.strand = strand
		return t.depth, nil
	}

	bp := strand.At(idx)
	if t.chld[bp] == nil {
		c := new(Trie)
		c.bp = byte(bp)
		t.chld[bp] = c
	}

	if d, e := t.chld[bp].add(strand, idx+1); e != nil {
		return 0, e
	} else {
		if d+1 > t.depth {
			t.depth = d + 1
		}
	}

	return t.depth, nil
}

func (t *Trie) Add(strand oligo.Oligo, idx int) (err error) {
	_, err = t.add(strand, idx)
	return
}

// Use this method with care. After it is called, the trie is no longer a tree,
// bug a DAG. It expects all the sequences to be of the same length (i.e.
// no t.strand to be non-nil if there are children in the trie. The trie t needs
// be a tree, not DAG. The method doesn't check for loops and will exhaust the
// stack.
// The method keeps track of up to pfx predecessors and append the appropriate
// trie from the tries array
func (t *Trie) Concat(pfxlen int, prefix uint64, tries []*Trie) int {
	if t == nil {
		return 0
	}

	ot := tries[prefix]
	t.depth = 0
	if t.strand != nil {
		if ot != nil {
			for i := 0; i < len(t.chld); i++ {
				if t.chld[i] != nil {
					panic("FIXME")
				}

				t.chld[i] = ot.chld[i]
//				if ot.chld[i] == nil {
//					fmt.Printf("pfx %d\n", prefix)
//				}

				if t.chld[i] != nil && t.depth < t.chld[i].depth + 1 {
					t.depth = t.chld[i].depth + 1
				}
			}
			t.strand = nil
		}
	} else {
		for i := 0; i < len(t.chld); i++ {
			if t.chld[i] != nil {
				pfx := (prefix<<2) | uint64(i)
				pfx &= (1<<(2*pfxlen) - 1)

				d := t.chld[i].Concat(pfxlen, pfx, tries) + 1
				if t.depth < d {
					t.depth = d
				}
			}
		}
	}

	return t.depth
}

func (t *Trie) SearchMin(seq oligo.Oligo, bporder []int, stoptime int64) (match *DistSeq) {
	var strand []byte

//	fmt.Printf("SearchMin %v\n", seq)
	if s, ok := seq.(*long.Oligo); ok {
		strand = s.Bytes()
	} else {
		strand = make([]byte, seq.Len())
		for i := 0; i < len(strand); i++ {
			strand[i] = byte(seq.At(i))
		}
	}

	count := CheckTimeCount
	match = t.searchMin(strand, bporder, stoptime, &count)
	return
}

func (t *Trie) searchMin(strand []byte, bporder []int, stoptime int64, count *int) (match *DistSeq) {
	distances := make([][]int, t.depth + 1)
	distances[0] = make([]int, len(strand) + 1)
	for i := 0; i < len(distances[0]); i++ {
		distances[0][i] = i
	}

	matchseq := make([]int, t.depth + 1)
//	fmt.Printf("search: trie %p word: %v chld %v bporder %v\n", t, strand, t.chld, bporder)
	maxdist := len(strand)
	didx := 0
	for n := 0; n < len(t.chld); n++ {
		i := n
		if bporder != nil {
			i = bporder[didx*len(t.chld) + n]
		}

		c := t.chld[i]
//		fmt.Printf("i %d c %d\n", i, c)
		if c == nil {
			continue
		}

		m := c.searchMinRecursive(i, strand, maxdist, didx, distances, matchseq, bporder, stoptime, count)
		if m != nil {
			match = m
			maxdist = m.Dist
		}
	}

	return
}

func newSeq(mseq []int) (ol oligo.Oligo) {
	ol = long.New(len(mseq))
	for i, nt := range mseq {
		ol.Set(i, nt)
	}

	return
}

func (t *Trie) searchMinRecursive(idx int, strand []byte, maxdist int, didx int, distances [][]int, matchseq []int, bporder []int, stoptime int64, count *int) (match *DistSeq) {
	if stoptime > 0 {
		if *count <= 0 {
			// if we already checked the time and we are over the limit, just return
			if *count < 0 {
				return
			}

			// we might be over the limit, check
			t := time.Now().UnixMilli()
//			fmt.Printf("time %d %d\n", t, stoptime)
			if t >= stoptime {
				*count = -1
				return
			}

			// not over the limit, reset the count and keep going
			*count = CheckTimeCount
		}
		*count--
	}

	colnum := len(strand) + 1

	// Build one row for the letter, with a column for each letter in the target
	// word, plus one for the empty string at column 0
	previousRow := distances[didx]
	currentRow := distances[didx + 1]
	if currentRow == nil {
		currentRow = make([]int, colnum)
		distances[didx + 1] = currentRow
	}

	currentRow[0] = previousRow[0] + 1
	rowMin := math.MaxUint32
	for col := 1; col < colnum; col++ {
		matchseq[didx] = idx
		insertCost := currentRow[col - 1] + 1
		deleteCost := previousRow[col] + 1
		replaceCost := previousRow[col - 1]
		if strand[col - 1] != byte(idx) {
			replaceCost++
		}

		minCost := insertCost
		if minCost > deleteCost {
			minCost = deleteCost
		}
		if minCost > replaceCost {
			minCost = replaceCost
		}

		currentRow[col] = minCost
		if rowMin > minCost {
			rowMin = minCost
		}
	}

//	fmt.Printf("didx %d idx %d maxdist %d xxx %d rowmin %d\n", didx, idx, maxdist, currentRow[colnum-1], rowMin)
	// if the last entry in the row indicates the optimal cost is less than the
	// maximum cost, and there is a word in this trie node, then add it.
	if currentRow[colnum - 1] < maxdist && t.strand != nil {
		match = new(DistSeq)
		match.Seq = newSeq(matchseq[0:didx + 1])
		match.Dist = currentRow[colnum - 1]
		maxdist = match.Dist
	}

	// if any entries in the row are less than the maximum cost, then 
	// recursively search each branch of the trie
	if rowMin < maxdist {
		var bpo [4]int

		didx++
		bpord := []int{ 0, 1, 2, 3 }
		n := didx*len(t.chld)
		if n < len(bporder) {
			bpord = bporder[n:n+4]
		}

		if didx < len(strand) {
			cidx := int(strand[didx])
			bpo[0] = cidx
			n := 1
			for _, i := range bpord {
				if i != cidx {
					bpo[n] = i
					n++
				}
			}
		} else {
			copy(bpo[:], bpord)
		}

//		fmt.Printf("bpo %v\n", bpo)
		for n := 0; n < len(t.chld); n++ {
			i := bpo[n]
			c := t.chld[i]
			if c == nil {
				continue
			}

			if m := c.searchMinRecursive(i, strand, maxdist, didx, distances, matchseq, bporder, stoptime, count); m != nil {
				match = m
				maxdist = m.Dist
			}
		}
	}

	return match
}

func (t *Trie) Size() (sz int) {
	t.visit(make(map[*Trie]bool), true, 0, func(tt *Trie, n float64) float64 {
		sz++
		return 0
	})

	return
}

func (root *Trie) SizeByDepth() (ret map[int]float64) {
	ret = make(map[int]float64)

	// first count how many times a node is referenced by other nodes
	tcnt := make(map[*Trie] uint64)
	root.visit(make(map[*Trie]bool), false, 0, func(t *Trie, n float64) float64 {
		tcnt[t]++
		return 0
	})

	// then visit each node again, but this time use the counts from before
	// to calculate the real number of nodes for each depth
	root.visit(make(map[*Trie]bool), true, 1, func(t *Trie, n float64) float64 {
		n *= float64(tcnt[t])
		ret[t.depth] += float64(n)
		return n
	})


	return
}

// calls the specified function once for each node in the trie
// uses the tmap parameter to keep track of the visited nodes
func (t *Trie) visit(tmap map[*Trie]bool, once bool, n float64, v func(t *Trie, n float64) float64) {
	if t == nil {
		return
	}

	if tmap[t] {
		// if the once flag is true, visit each reference of the node
		// but not its children
		if !once {
			v(t, n)
		}
		return
	}

	tmap[t] = true
	m := v(t, n)
	for _, c := range t.chld {
		c.visit(tmap, once, m, v)
	}

	return
}


func (t *Trie) Depth() int {
	return t.depth
}

func (t *Trie) Clone() (ret *Trie) {
	if t == nil {
		return nil
	}

	ret = new(Trie)
	ret.bp = t.bp
	ret.depth = t.depth
	ret.strand = t.strand
	for i := 0; i < len(t.chld); i++ {
		ret.chld[i] = t.chld[i].Clone()
	}

	return
}
