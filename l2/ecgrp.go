package l2

import (
	"fmt"
	"os"
	"sync"
	"github.com/klauspost/reedsolomon"
)

type Blk struct {
	b	[]byte
	n	int
}

type Blkset []Blk

// Represent a data block from the original encoding
type EcElem struct {
	bset	Blkset		// blocks collected so far
	vdata	Blkset		// verified data
	uvdata	Blkset		// unverified data
}

// Each column in the ECGroup has its erasure data calculated separately
// The first dseqnum elements represent data, with the last rseqnum being
// the erasure codes
type EcCol struct {
	elems	[]EcElem
}

// The ECGroup is a 2D array of blknum columns and dseqnum+rseqnum rows
// We represent it as an array of columns
type EcGroup struct {
	sync.Mutex
	cols	[]EcCol
	verbose	bool		// for debugging
}

const (
	// EcCol status bits
	Verified = 1 << iota
)

func (db Blk) Bytes() []byte {
	return db.b
}

func (db Blk) Count() int {
	return db.n
}


// Checks if db is in the set of blocks
func (ds Blkset) Find(d []byte) *Blk {
	if ds == nil {
			return nil
	}

	for idx := range ds {
		b := &ds[idx]
		match := true
		for i, v := range b.b {
			if v != d[i] {
				match = false
				break
			}
		}

		if match {
			return b
		}
	}

	return nil
}

// Add block to the set of blocks
func (ds Blkset) Add(d []byte) (ret Blkset, added bool) {
	if b := ds.Find(d); b != nil {
		b.n++
		ret = ds
	} else {
		ret = append([]Blk(ds), Blk{d, 1})
		added = true
	}

	return
}

// Convert the set to slice of blocks
func (ds Blkset) Blks() []Blk {
	return []Blk(ds)
}

func (ds Blkset) Best() (ret []Blk) {
	max := 0
	for idx := range ds {
		b := &ds[idx]
		if b.n < max {
			continue
		}

		if b.n > max {
			max = b.n
			ret = nil
		}

		ret = append(ret, *b)
	}

	return
}

func newEcGroup(rows, cols int) (eg *EcGroup) {
	eg = new(EcGroup)
	eg.cols = make([]EcCol, cols)

	for i := 0; i < cols; i++ {
		eg.cols[i].elems = make([]EcElem, rows)
	}

	return eg
}

// Add data from another oligo to the EC group
// Returns true if there is a change in the EC group data
func (eg *EcGroup) addEntry(row int, dblks []Blk, ecnum int, rsenc reedsolomon.Encoder) (ret bool) {
	if eg.verbose {
		fmt.Fprintf(os.Stderr, "--- addEntry %d %v\n", row, dblks)
	}

	eg.Lock()
	maxblk := len(dblks)
//	fmt.Fprintf(os.Stderr, "addEntry %d\n", row)
	for i, db := range dblks {
		col := ecGroupReverseColumn(i, row, maxblk)
		success := eg.addBlock(row, col, db, ecnum, rsenc)
		ret = ret || success
	}
	eg.Unlock()

	return
}

// Add a block to the element with the specified coordinates
// Try to recover more data with the newly added block, if possible
// Returns true if there is a change in the data recovery
func (eg *EcGroup) addBlock(row, col int, db Blk, ecnum int, rsenc reedsolomon.Encoder) (ret bool) {
//	fmt.Fprintf(os.Stderr, "\taddBlock %d %v\n", col, db)
	// if the block is empty, don't bother
	if db.Bytes() == nil {
		return false
	}

	c := eg.cols[col]

	// check if the block is already present
	if bs, added := c.elems[row].bset.Add(db.b); !added {
//		if eg.verbose {
//			fmt.Fprintf(os.Stderr, "-+- row %d col %d %v %v\n", row, col, db, c.elems[row].bset)
//		}

		return false
	} else {
		c.elems[row].bset = bs
	}

	if eg.verbose {
		fmt.Fprintf(os.Stderr, "+++ column %d\n", col)
		for i := 0; i < len(c.elems); i++ {
			eblks := c.elems[i].bset.Blks()
			fmt.Fprintf(os.Stderr, "\t%v\n", eblks)
		}
	}
	// go over all combinations of data blocks
	shards := make([][]byte, len(c.elems))
	idx := make([]int, len(c.elems))
	shards[row] = db.Bytes()		// always use db at the row position
	rownum := len(c.elems)
	dseqnum := rownum - ecnum		// number of data sequences
	for done := false; !done; {
//		fmt.Printf("\t\tidx %v\n", idx)
		// collect the data from the current combination
		nshards := 1	// non-nil shards
		nec := 0
		if row >= dseqnum {
			nec++
		}

		for i, m := range idx {
			if i == row {
				continue
			}

			eblks := c.elems[i].bset.Blks()
			if m < len(eblks) {
				shards[i] = eblks[m].Bytes()
				nshards++
				if i >= dseqnum {
					nec++
				}
			} else {
				shards[i] = nil
			}
		}

		// setup the indices for the next combination
		for i := 0; i < len(idx); i++ {
			if i == row {
				// skip the row with the new data
				done = i+1 == len(idx)
				continue
			}

			eblks := c.elems[i].bset.Blks()
			idx[i]++
			if idx[i] < len(eblks) {
				break
			} else {
				idx[i] = 0

				// check if we exhausted all combinations
				done = i+1 == len(idx)
			}
		}

//		fmt.Printf("\t\tnshards %d nec %d %v\n", nshards, nec, shards)
		verified := true
		if (rownum - nshards) * 2 > ecnum {
			// we have more missing shards than we can recover from
			// try anyway, but store in the unverified pile
			verified = false
		}

/*
		if nec == 0 {
			// We don't have any erasure shards, so we can't check if the
			// data shards contain errors. Still, if we have all the data
			// shards, return them flagging that they might be wrong
			if nshards == dseqnum {
				// copy the data into the unverified set
				for i := 0; i < len(c.elems); i++ {
					if !c.elems[i].uvdata.Exist(shards[i]) {
						c.elems[i].uvdata = c.elems[i].uvdata.Add(shards[i])
					}
				}

				// report that something changed when done
				ret = true
				continue
			}

			// otherwise, skip the rest of the code for this combination
			continue
		}
*/

		err := rsenc.Reconstruct(shards)
		if err == nil {
			var ok bool
			ok, err = rsenc.Verify(shards)
			if err == nil && !ok {
				err = Eec
			}
		}

		if eg.verbose {
			fmt.Fprintf(os.Stderr, "=== col %d %v %v %v\n", col, shards, verified, err)
		}

		if err != nil {
//			fmt.Printf("\t\terr %v\n", err)
			continue
		}

		// copy the data to the appropriate set		
		for i := 0; i < len(c.elems); i++ {
			if verified {
				var added bool
				c.elems[i].vdata, added = c.elems[i].vdata.Add(shards[i])
				if added && eg.verbose {
					fmt.Fprintf(os.Stderr, "\tVVV %d %v\n", i, c.elems[i].vdata)
				}
			} else {
				var added bool
				c.elems[i].uvdata, added = c.elems[i].uvdata.Add(shards[i])

				if added && eg.verbose {
					fmt.Fprintf(os.Stderr, "\tUUU %d %v\n", i, c.elems[i].uvdata)
				}
			}
		}

		// report that something changed when done
		ret = true
	}

	return
}

// Returns all the verified data for the specified element in the EcGroup
func (eg *EcGroup) getVerified(row, col int) (blks []Blk) {
	if eg == nil {
		return nil
	}

	eg.Lock()
	cidx := ecGroupReverseColumn(col, row, len(eg.cols))
	c := eg.cols[cidx]
	el := &c.elems[row]
	blks = el.vdata.Blks()
	eg.Unlock()

	if eg.verbose {
		fmt.Fprintf(os.Stderr, "getVerified %d %d: cidx %d: %v\n", row, col, cidx, blks)
	}
	return
}

// Returns the unverified data for the specified element in the EcGroup
func (eg *EcGroup) getUnverified(row, col int) (blks []Blk) {
	if eg == nil {
		return nil
	}

	eg.Lock()
	cidx := ecGroupReverseColumn(col, row, len(eg.cols))
	c := eg.cols[cidx]
	el := &c.elems[row]
	blks = el.uvdata.Blks()
	eg.Unlock()

	if eg.verbose {
		fmt.Fprintf(os.Stderr, "getUnverified %d %d: cidx %d: %v\n", row, col, cidx, blks)
	}
	return
}

// Returns the best possible data in case that we can't even recover unverified data
// "The best" means the data from the block with the highest count (if one).
func (eg *EcGroup) getBestGuess(row, col int) (blks []Blk) {
	if eg == nil {
		return nil
	}

	eg.Lock()

	cidx := ecGroupReverseColumn(col, row, len(eg.cols))
	c := eg.cols[cidx]
	el := &c.elems[row]
	blks = el.bset.Best()
	eg.Unlock()

	if eg.verbose {
		fmt.Fprintf(os.Stderr, "getBestGuess %d %d: cidx %d: %v\n", row, col, cidx, blks)
	}
	return
}

// Returns true of the EcGroup has all the data
func (eg *EcGroup) isComplete(verified bool) bool {
	eg.Lock()
	defer eg.Unlock()
	for _, c := range eg.cols {
		for r := 0; r < len(c.elems); r++ {
			el := &c.elems[r]
			if verified {
				if len(el.vdata.Blks()) == 0 {
					return false
				}
			} else {
				if len(el.uvdata.Blks()) == 0 {
					return false
				}
			}
		}
	}

	return true
}

// Calculate the column of a block with position p from row r, with up to maxblks per row
func ecGroupGetColumn(p, r, maxblks int) int {
	return  (p + r) % maxblks	// combine blocks diagonally (standard for ADS Codex)
//	return p			// combine blocks vertically
}

// Reverse of ecGroupGetColumn
func ecGroupReverseColumn(c, r, maxblks int) int {
	n := (c - r) % maxblks
	if n < 0 {
		n += maxblks
	}

	return n
}

func ecGroupDataSize(dblksz, dblknum, dseqnum int) int {
	return dblksz * dblknum * dseqnum
}

// Encodes the specified data into a list of data blocks that can be fed to the L1 encoder
// The first index is the row, the second is the column within the row
func ecGroupEncode(dblksz, dblknum, dseqnum, eseqnum int, rsenc reedsolomon.Encoder, data []byte) (dblks [][][]byte, err error) {
	ecsz := ecGroupDataSize(dblksz, dblknum, dseqnum)
	if len(data) != ecsz {
		err = fmt.Errorf("invalid data size for EC group: expected %d got %d", ecsz, len(data))
		return
	}

//	fmt.Printf("data %v\n", data)
	dblks = make([][][]byte, dseqnum + eseqnum)
	shards := make([][]byte, dseqnum + eseqnum)
	for r := 0; r < dseqnum + eseqnum; r++ {
		shards[r] = make([]byte, dblksz)
		dblks[r] = make([][]byte, dblknum)
		for c := 0; c < dblknum; c++ {
			dblks[r][c] = make([]byte, dblksz)
			if (r < dseqnum) {
				n := (r * dblknum + c) * dblksz
				copy(dblks[r][c], data[n:n + dblksz])
			}
		}
	}

	for col := 0; col < dblknum; col++ {
		// put the data in the shard
		for row := 0; row < dseqnum; row++ {
			pos := ecGroupGetColumn(col, row, dblknum)
			n := (row * dblknum + pos) * dblksz
			dblk := data[n:n+4]
			copy(shards[row], dblk)
		}

		// calculate the erasure data
		err = rsenc.Encode(shards)
		if err != nil {
			return
		}

//		fmt.Printf("col %d: %v\n", col, shards)

		// put the erasure data where it belongs
		for row := dseqnum; row < dseqnum + eseqnum; row++ {
			pos := ecGroupGetColumn(col, row, dblknum)
			copy(dblks[row][pos], shards[row])
		}

	}

	return
}

// Doesn't really decode the group, but puts the encoded EcGroup elements data together
// The first index of els is the row, the second is the column
func ecGroupDecode(offset uint64, elems [][][]byte) (dss []DataExtent) {
	var d []byte
	off := offset
	for _, row := range elems {
		for _, el := range row {
			if len(d) != 0 && off+uint64(len(d)) != offset {
				dss = append(dss, DataExtent{ off, d, FileHole })
				off = offset
				d = nil
			}

			offset += uint64(len(el))
			d = append(d, el...)
		}
	}

	return
}
