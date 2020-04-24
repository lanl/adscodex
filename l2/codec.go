package l2

import (
	"errors"
_	"fmt"
	"math/rand"
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
	var failed []oligo.Oligo
	var last, l uint64

	doligos := make(map[uint64] [][]byte)	// data oligos
	eoligos := make(map[uint64] [][]byte)	// erasure oligos

	// first try without forcing metadata recovery at L1
	last, failed = c.decodeOligos(start, end, oligos, doligos, eoligos, false)
	data = c.recoverData(start, last, doligos, eoligos)
	if len(data) == 1 {
		// we recovered all the data, return
		goto done
	}

	// Now try harder. We processed all good oligos, try to process only the bad ones
	l, _ = c.decodeOligos(start, end, failed, doligos, eoligos, true)
	if l > last {
		last = l
	}
	data = c.recoverData(start, last, doligos, eoligos)

done:
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
func (c *Codec) decodeOligos(start, end uint64, oligos []oligo.Oligo, doligos, eoligos map[uint64][][]byte, tryhard bool) (last uint64, failed []oligo.Oligo) {
	for _, o := range oligos {
		addr, ef, d, err := c.c1.Decode(c.p5, c.p3, o, tryhard)
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

				failed = append(failed, o)
			} else {
				panic("unknown error")
			}
		}

		if addr < start || addr >= end {
			continue
		}

		if addr > last {
			last = addr
		}

		if ef {
			if eoligos[addr] != nil {
				panic("duplicate addresses")
			}

			eoligos[addr] = d
		} else {
			if doligos[addr] != nil {
				panic("duplicate addresses")
			}

			doligos[addr] = d
		}
	}

	return
}

// Combine oligos in erasure groups and extract the data from them.
// If there are ranges of data that are unrecoverable, return multiple
// extents with all the data that we were able to recover
func (c *Codec) recoverData(start, end uint64, doligos, eoligos map[uint64][][]byte) (data []DataExtent) {
	egrp := make([][][]byte, c.dseqnum + c.rseqnum)
	for a := start; a < end; a += uint64(c.dseqnum) {
		for i := 0; i < c.dseqnum; i++ {
			egrp[i] = doligos[a + uint64(i)]
			if egrp[i] == nil {
				// if the whole oligo is missing, put nils for the whole row
				egrp[i] = make([][]byte, c.dblknum)
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
				egrp[n] = make([][]byte, c.dblknum)
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

func (c *Codec) recoverECGroup(offset uint64, egrp [][][]byte) (ds []DataExtent) {
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

		err := c.ec.ReconstructData(shards)
		if err == nil {
			// reconstruction failed
			// TODO: if we have enough a lot of data
			// we can try setting random shards to nil and
			// trying to reconstruct without them
			continue
		}

		// move the reconstructed data back to the egrp array
		for i := 0; i < c.dseqnum; i++ {
			p := (n + i) % c.dblknum
			if p >= c.dblknum {
				p = 0
			}

			egrp[i][p] = shards[i]
		}
	}

	// combine the data into data extents
	var data []byte
	off := offset
	for i := 0; i < c.dseqnum; i++ {
		for _, b := range egrp[i] {
			if b != nil {
				if off + uint64(len(data)) != offset {
					// start new extent
					ds = append(ds, DataExtent{ off, data })
					off = offset
					data = nil
				}

				data = append(data, b...)
			}

			offset += uint64(len(b))
		}
	}

	if data != nil {
		ds = append(ds, DataExtent{ off, data })
	}
	
	return
}
