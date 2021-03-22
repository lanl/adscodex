package utils

import (
_	"fmt"
_	"errors"
	"math"
	"adscodex/oligo"
	"adscodex/oligo/long"
)

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

func NewTrie(seqs []*Oligo) (trie *Trie, err error) {
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

func (t *Trie) Search(seq oligo.Oligo, dist int) (match []DistSeq) {
	var strand []byte

	if s, ok := seq.(*Oligo); ok {
		seq = s.ol
	}

	if s, ok := seq.(*long.Oligo); ok {
		strand = s.Bytes()
	} else {
		strand = make([]byte, seq.Len())
		for i := 0; i < len(strand); i++ {
			strand[i] = byte(seq.At(i))
		}
	}

	return t.search(strand, dist, match)
}

func (t *Trie) search(strand []byte, maxdist int, match []DistSeq) []DistSeq {
	distances := make([][]int, t.depth + 1)
	distances[0] = make([]int, len(strand) + 1)
	for i := 0; i < len(distances[0]); i++ {
		distances[0][i] = i
	}

//	fmt.Printf("search: trie %p maxdist %d word: %v\n", t, maxdist, strand)
	for i, c := range t.chld {
		if c == nil {
			continue
		}

		match = c.searchRecursive(i, strand, maxdist, 0, distances, match)
	}

	return match
}

func (t *Trie) searchRecursive(idx int, strand []byte, maxdist int, didx int, distances [][]int, match []DistSeq) []DistSeq {
	colnum := len(strand) + 1

	// Build one row for the letter, with a column for each letter in the target
	// word, plus one for the empty string at column 0
//	fmt.Printf("searchRecursive: %p previous: %v\n", t, previousRow)
	previousRow := distances[didx]
	currentRow := distances[didx + 1]
	if currentRow == nil {
		currentRow = make([]int, colnum)
		distances[didx + 1] = currentRow
	}

	currentRow[0] = previousRow[0] + 1
	rowMin := math.MaxUint32
	for col := 1; col < colnum; col++ {
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
//	fmt.Printf("current: min %d %v\n", rowMin, previousRow)

	// if the last entry in the row indicates the optimal cost is less than the
	// maximum cost, and there is a word in this trie node, then add it.
	if currentRow[colnum - 1] <= maxdist && t.strand != nil {
		match = append(match, DistSeq { t.strand, currentRow[colnum - 1] })
	}

	// if any entries in the row are less than the maximum cost, then 
	// recursively search each branch of the trie
	if rowMin <= maxdist {
//		fmt.Printf("+++ search children %p: [", t)
//		for _, c := range t.chld {
//			fmt.Printf("%p ", c)
//		}
//		fmt.Printf("]\n")
		for i, c := range t.chld {
			if c == nil {
				continue
			}

			match = c.searchRecursive(i, strand, maxdist, didx + 1, distances, match)
		}
	} else {
//		fmt.Printf("+++ skip children %p\n", t)
	}

	return match
}











func (t *Trie) SearchMin(seq oligo.Oligo) (match *DistSeq) {
	var strand []byte

	if s, ok := seq.(*Oligo); ok {
		seq = s.ol
	}

	if s, ok := seq.(*long.Oligo); ok {
		strand = s.Bytes()
	} else {
		strand = make([]byte, seq.Len())
		for i := 0; i < len(strand); i++ {
			strand[i] = byte(seq.At(i))
		}
	}

	match = t.searchMin(strand)
	return
}

func (t *Trie) searchMin(strand []byte) (match *DistSeq) {
	distances := make([][]int, t.depth + 1)
	distances[0] = make([]int, len(strand) + 1)
	for i := 0; i < len(distances[0]); i++ {
		distances[0][i] = i
	}

//	fmt.Printf("search: trie %p maxdist %d word: %v\n", t, maxdist, strand)
	maxdist := len(strand)
	for i, c := range t.chld {
		if c == nil {
			continue
		}

		m := c.searchMinRecursive(i, strand, maxdist, 0, distances)
		if m != nil {
			match = m
			maxdist = m.Dist
		}
	}

	return
}

func (t *Trie) searchMinRecursive(idx int, strand []byte, maxdist int, didx int, distances [][]int) (match *DistSeq) {
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

	// if the last entry in the row indicates the optimal cost is less than the
	// maximum cost, and there is a word in this trie node, then add it.
	if currentRow[colnum - 1] < maxdist && t.strand != nil {
		match = new(DistSeq)
		match.Seq = t.strand
		match.Dist = currentRow[colnum - 1]
		maxdist = match.Dist
	}

	// if any entries in the row are less than the maximum cost, then 
	// recursively search each branch of the trie
	if rowMin < maxdist {
		for i, c := range t.chld {
			if c == nil {
				continue
			}

			if m := c.searchMinRecursive(i, strand, maxdist, didx + 1, distances); m != nil {
				match = m
				maxdist = m.Dist
			}
		}
	}

	return match
}

func (t *Trie) Size() int {
	if t == nil {
		return 0
	}

	return 1 + t.chld[0].Size() + t.chld[1].Size() + t.chld[2].Size() + t.chld[3].Size()
}
