package l2

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"acoma/oligo"
	"acoma/criteria"
	"acoma/l0"
	"acoma/l1"
	"github.com/klauspost/reedsolomon"
)

// Level 2 codec
type Codec struct {
	p5, p3	oligo.Oligo	// 5'-end and 3'-end primers
	dblknum int		// number of data blocks in an oligo
	dseqnum	int		// number of data sequences
	rseqnum	int		// number of erasure sequences

	c1	*l1.Codec
	ec	reedsolomon.Encoder
}

// Data extent
// Describes a sequential range of data recovered
type DataExtent struct {
	Offset	uint64
	Data	[]byte
}

type doligo struct {
	addr	uint64
	ef	bool
	data	[][]byte	// if nil (and failed is also nil), end of goroutine
	failed	oligo.Oligo	// we couldn't decode it
}

var Eec = errors.New("parity blocks don't match")

// Creates a new L2 codec
// Parameters:
//	p5	5'-end primer
//	p3	3'-end primer
//	dblknum	number of data blocks in an oligo
//	mdsz	size of a metadata block in an oligo
//	mdrsnum	number of Reed-Solomon metadata erasure blocks
//	dseqnum	number of data oligos in the erasure block
//	rseqnum	number of erasure oligos in the erasure block
func NewCodec(p5, p3 oligo.Oligo, dblknum, mdsz, mdrsnum, dseqnum, rseqnum int) *Codec {
	var err error

	c := new(Codec)
	c.p5 = p5
	c.p3 = p3
	c.dblknum = dblknum
	c.dseqnum = dseqnum
	c.rseqnum = rseqnum
	c.c1 = l1.NewCodec(dblknum, mdsz, mdrsnum, criteria.H4G2)
	c.ec, err = reedsolomon.New(dseqnum, rseqnum)

	if err != nil {
		panic("reedsolomon error")
	}

	return c
}

// Encodes logical data into a collection of oligos.
// The addr parameter specifies the starting address for the data
// The data parameter points to the data to encode.
// The function returns the next available address as well as an
// array of oligos that encode the data.
// If the data is not aligned, it is padded with random values at
// the end. The size of the data is stored in the last 8 bytes
// encoded in the last oligo.
func (c *Codec) Encode(addr uint64, data []byte) (nextaddr uint64, oligos []oligo.Oligo, err error) {
	blknum := c.c1.BlockNum()
	blksz := c.c1.BlockSize()

	// first pad the data at the back so it's multiple of the data per erasure group
	// (the length of the data is encoded in the last 8 bytes
	dsz := uint64(len(data))
	oligosz := blknum * blksz
	egsz := oligosz * c.dseqnum
	if (len(data) + 8)%egsz != 0 {
		n := egsz - ((len(data) + 8) % egsz)
		for i := 0; i < n; i++ {
			data = append(data, byte(rand.Int31n(256)))
		}
	}
	data = l0.Pint64(dsz, data)

	// first index is the place in the erasure group
	// second index is the data block within the oligo
	// third index is the byte within the data block
	egrp := make([][][]byte, c.dseqnum + c.rseqnum)
	for i := 0; i < len(egrp); i++ {
		egrp[i] = make([][]byte, blknum)

		// allocate memory for the data of the erasure sequences
		if i >= c.dseqnum {
			for j := 0; j < len(egrp[i]); j++ {
				egrp[i][j] = make([]byte, blksz)
			}
		}
	}

	for len(data) > 0 {
		// populate the data blocks for an erasure group
		for i := 0; i < c.dseqnum; i++ {
			for j := 0; j < blknum; j++ {
				egrp[i][j] = data[0:blksz]
				data = data[blksz:]
			}
		}

		// generate the data for the erasure oligos
		err = c.generateErasures(egrp)
		if err != nil {
			return
		}

		// encode the data oligos for an erasure group
		for i := 0; i < c.dseqnum; i++ {
			var o oligo.Oligo

			o, err = c.c1.Encode(c.p5, c.p3, addr + uint64(i), false, egrp[i])
			if err != nil {
				return
			}

			oligos = append(oligos, o)
		}

		// encode the erasure oligos for an erasure group
		for i := 0; i < c.rseqnum; i++ {
			var o oligo.Oligo

			o, err = c.c1.Encode(c.p5, c.p3, addr + uint64(i), true, egrp[i+c.dseqnum])
			if err != nil {
				return
			}
			oligos = append(oligos, o)
		}

		addr += uint64(c.dseqnum)
	}

	nextaddr = addr
	return
}

// Decodes oligos with addresses from start to end.
// The oligos array may contain extra oligo sequences that are not used.
// Return all data that we recovered in data extents
func (c *Codec) Decode(start, end uint64, oligos []oligo.Oligo) (data []DataExtent) {
	var last uint64

	// maps from oligo addresses to a list of data blocks for that address
	// The first array of the entry is always dblknum elements long len([][][]byte) == dblknum
	// The second array contains all different values for the data block at that place of the oligo
	// The third array is always 4 bytes long and represents the content of the data block
	doligos := make(map[uint64] [][][]byte)	// data oligos (list per address)
	eoligos := make(map[uint64] [][][]byte)	// erasure oligos (list per address)

	reoligos := oligos
	for dfclty := 0; dfclty < 2; dfclty++ {
		l, f := c.decodeOligos(start, end, reoligos, doligos, eoligos, dfclty)
//		fmt.Printf("difficulty: %d total %d failed %d\n", dfclty, len(reoligos), len(f))

		reoligos = f
		if l > last {
			last = l
		}

		data = c.recoverData(start, last, doligos, eoligos)
		if len(data) == 1 {
			// we recovered all the data, return
			break
		}

/*		{
			n := 0
			for _, de := range data {
				n += len(de.Data)
			}

			fmt.Printf("\tgot %d extents, %d/%d bytes\n", len(data), n, last * uint64(c.c1.DataLen()))
		}
*/
	}

	if len(data) != 0 {
		var dsz uint64

		le := &data[len(data) - 1]

		if len(le.Data) > 8 {
		        // retrieve the size of the data from the last 8 bytes
        		dsz, _ = l0.Gint64(le.Data[len(le.Data) - 8:])
		}

		if dsz > le.Offset && (dsz-le.Offset) < uint64(len(le.Data)) {
			le.Data = le.Data[0:dsz - le.Offset]
		}
	}

	// return whatever we recovered, hopefully everything
	return
}

// Decodes the oligos and puts the ones with addresses from start to end into the
// appropriate maps, depending if the contain data or erasure codes.
// If the parameter tryhard is true, tells the L1 codec to try harder to recover
// the metadata
func (c *Codec) decodeOligos(start, end uint64, oligos []oligo.Oligo, doligos, eoligos map[uint64][][][]byte, difficulty int) (last uint64, failed []oligo.Oligo) {
	// Decode in parallel
	procnum := runtime.NumCPU()
	olperproc := 1 + len(oligos)/procnum
	if olperproc < 100 {
		// don't spin goroutines if there are not that many oligos
		procnum = 1
		olperproc = len(oligos)
	}

	ch := make(chan doligo, procnum)
	for i := 0; i < procnum; i++ {
		istart := i * olperproc
		if istart >= len(oligos) {
			procnum = i
			break
		}

		iend := (i+1) * olperproc
		if iend > len(oligos) {
			iend = len(oligos)
		}

		go func(s, e int, oligos []oligo.Oligo) {
			var do doligo

			ols := oligos[s:e]
			for _, o := range ols {
				var err error

				do.failed = nil
				do.addr, do.ef, do.data, err = c.c1.Decode(c.p5, c.p3, o, difficulty)
				if err != nil {
					if err == l1.Eprimer {
						// one of the primers didn't match, just discard the oligo
						continue
					} else if err == l1.Emetadata {
						// We couldn't recover the metadata without hard work,
						// save the oligo for later
						// First we are going to try to recover the data using the
						// erasure oligos, if that fails, we'll have to try harder
						// to recover the metadata and try again

						do.failed = o
					} else {
						panic(fmt.Sprintf("unknown error: %v", err))
					}
				}

				if do.addr < start || do.addr >= end {
					continue
				}

				ch <- do
			}

			do.data = nil
			do.failed = nil
			ch <- do
		} (istart, iend, oligos)
	}

	for doneprocs := 0; doneprocs < procnum; {
		do := <- ch
		if do.data == nil && do.failed == nil {
			doneprocs++
			continue
		}

		if do.failed != nil {
			failed = append(failed, do.failed)
			continue
		}

		addr, ef, d := do.addr, do.ef, do.data
		if addr > last {
			last = addr
		}

		var dd [][][]byte
		if ef {
			dd = eoligos[addr]
		} else {
			dd = doligos[addr]
		}

		if dd == nil {
			dd = make([][][]byte, c.dblknum)
		}

		// for each data block from the new oligo, add it to the appropriate place,
		// not nil and if not already there
		for i := 0; i < c.dblknum; i++ {
			di := d[i]
			if di == nil {
				// no data, skip
				continue
			}

			add := dd[i] == nil
			ddi := dd[i]
			for j := 0; !add && j < len(ddi); j++ {
				ddj := ddi[j]
				for n := 0; n < len(ddj); n++ {
					if ddj[n] != di[n] {
						add = true
						break
					}
				}
			}

			if add {
				dd[i] = append(dd[i], d[i])
			}
		}

		if ef {
			eoligos[addr] = dd
		} else {
			doligos[addr] = dd
		}
		
	}

	return
}

// Combine oligos in erasure groups and extract the data from them.
// If there are ranges of data that are unrecoverable, return multiple
// extents with all the data that we were able to recover
func (c *Codec) recoverData(start, end uint64, doligos, eoligos map[uint64][][][]byte) (data []DataExtent) {
	egrp := make([][][][]byte, c.dseqnum + c.rseqnum)
	for a := start; a < end; a += uint64(c.dseqnum) {
		for i := 0; i < c.dseqnum; i++ {
			egrp[i] = doligos[a + uint64(i)]
			if egrp[i] == nil {
				// if the whole oligo is missing, put nils for the whole row
				egrp[i] = make([][][]byte, c.dblknum)
				for j := 0; j < c.dblknum; j++ {
					egrp[i][j] = nil
				}
			}
		}

		for i := 0; i < c.rseqnum; i++ {
			n := i + c.dseqnum
			egrp[n] = eoligos[a + uint64(i)]
			if egrp[n] == nil {
				// if the whole oligo is missing, put nils for the whole row
				egrp[n] = make([][][]byte, c.dblknum)
				for j := 0; j < c.dblknum; j++ {
					egrp[n][j] = nil
				}
			}
		}

		ds := c.recoverECGroup(a * uint64(c.c1.DataLen()), egrp)

		// check if the first extent can be combined with the last one so far
		if data != nil && ds != nil {
			last := &data[len(data) - 1]
			if last.Offset + uint64(len(last.Data)) == ds[0].Offset {
				last.Data = append(last.Data, ds[0].Data...)
				ds = ds[1:]
			}
		}

		// append the data extents
		data = append(data, ds...)
	}

	return
}

func (c *Codec) generateErasures(egrp [][][]byte) error {
	shards := make([][]byte, c.dseqnum + c.rseqnum)

	// each data block within the oligo is processed separately
	for n := 0; n < c.dblknum; n++ {
		// collect the slices for the blocks based on a diagonal pattern
		for i := 0; i < c.dseqnum + c.rseqnum; i++ {
			p := (n + i) % c.dblknum
			if p >= c.dblknum {
				p = 0
			}

			shards[i] = egrp[i][p]
		}

		// calculate the RS erasures
		err := c.ec.Encode(shards)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Codec) verifyErasures(egrp [][][]byte) error {
	shards := make([][]byte, c.dseqnum + c.rseqnum)

	// each data block within the oligo is processed separately
	for n := 0; n < c.dblknum; n++ {
		// collect the slices for the blocks based on a diagonal pattern
		for i := 0; i < c.dseqnum + c.rseqnum; i++ {
			p := (n + i) % c.dblknum
			if p >= c.dblknum {
				p = 0
			}

			shards[i] = egrp[i][p]
		}

		ok, err := c.ec.Verify(shards)
		if err != nil {
			return err
		} else if !ok {
			return errors.New("erasure blocks don't match")
		}
	}

	return nil
}

// egrp is an erasure group of dseqnum+rseqnum oligos
// The first index is the place of the oligo in the group
// The second index is [0,blknum) and represents the data block in the oligo
// The third index iterates through all possible content of the data blocks from various oligos with the same address
// The fourth index is [0,4) and represents the content of the data block
func (c *Codec) recoverECGroup(offset uint64, egrp [][][][]byte) (ds []DataExtent) {
	shards := make([][]byte, c.dseqnum + c.rseqnum)		// Reed-Solomon shards
	idx := make([]int, c.dseqnum + c.rseqnum)		// indices for each member of the group (if there are multiple data blocks per member)
	dblks := make([][][]byte, c.dseqnum + c.rseqnum)
	data := make([][][]byte, c.dseqnum)			// reconstructed data blocks for each position in the erasure group

	for i := 0; i < len(data); i++ {
		data[i] = make([][]byte, c.dblknum)
	}

//	for i, m := range egrp {
//		fmt.Printf("%d: %v\n", i, m)
//	}

	// each data block within the oligo is processed separately
	for n := 0; n < c.dblknum; n++ {
		// setup the data blocks from the erasure group based on a diagonal pattern
		for i := 0; i < len(idx); i++ {
			p := (n + i) % c.dblknum
			if p >= c.dblknum {
				p = 0
			}

			idx[i] = 0
			dblks[i] = egrp[i][p]
		}

		// go over all combinations of data blocks
		var done bool
		for !done {
			// collect the current combination
//			fmt.Printf("%d idx %v\n", offset, idx)
			nshards := 0
			for i, m := range idx {
				if len(dblks[i]) > m {
					shards[i] = dblks[i][m]
					nshards++
				} else {
					shards[i] = nil
				}
			}

			// setup the indices for the next combination
			var i int
			for i = 0; i < len(idx); i++ {
				idx[i]++
				if idx[i] < len(dblks[i]) {
					break
				} else {
					// check if we exhausted all combinations
					if i+1 == len(idx) {
						done = true
					}
				}
			}

//			fmt.Printf("%d <<< shard %d: %v\n", offset, n, shards)
			err := c.ec.Reconstruct(shards)
			if err == nil {
				var ok bool

				ok, err = c.ec.Verify(shards)
				if err == nil && !ok {
					err = Eec
				}
			}

//			fmt.Printf("%d >>> shard %d: %v err %v\n", offset, n, shards, err)
			if err != nil {
				// The reconstruction failed, but if we had too many non-nil blocks
				// we can try removing some of them and retrying.
				if nshards < c.dseqnum {
					// if we have too few non-nil shards, there is no point of even trying
					continue
				}

				// Keep it simple for now, remove only one shard and retry
				// TODO: eventually make it reflect the number of erasure shards
				for i := 0; i < len(shards); i++ {
					tshard := shards[i]
					shards[i] = nil

					err := c.ec.Reconstruct(shards)
					if err == nil {
						var ok bool

						ok, err = c.ec.Verify(shards)
						if err == nil && !ok {
							err = Eec
						}
					}

					if err == nil {
						// we found a combination that works
						goto done
					}

					shards[i] = tshard
				}

				// we failed
				continue
			}

done:
			// move the reconstructed data into an array to recombine
			// into extents later
			for i := 0; i < c.dseqnum; i++ {
				p := (n + i) % c.dblknum
				if p >= c.dblknum {
					p = 0
				}

				data[i][p] = shards[i]
			}

			// since we found a match, skip the rest of the combinations
			break
		}
	}

	// combine the data into data extents
	var d []byte
	off := offset
	for i := 0; i < c.dseqnum; i++ {
		for _, b := range data[i] {
			if b != nil {
				if off + uint64(len(d)) != offset {
					// start new extent
					ds = append(ds, DataExtent{ off, d })
					off = offset
					d = nil
				}

				d = append(d, b...)
			}

			offset += uint64(len(b))
		}
	}

	if d != nil {
		ds = append(ds, DataExtent{ off, d })
	}

//	fmt.Printf("ds %v\n", ds)	
	return
}
