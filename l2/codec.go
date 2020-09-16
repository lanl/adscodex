package l2

import (
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"crypto/sha1"
	"hash/crc64"
	"os"
	"acoma/oligo"
	"acoma/criteria"
	"acoma/l0"
	"acoma/l1"
	"github.com/klauspost/reedsolomon"
)

// Level 2 codec
type Codec struct {
	// settings
	p5, p3	oligo.Oligo	// 5'-end and 3'-end primers
	dblknum int		// number of data blocks in an oligo
	dseqnum	int		// number of data sequences
	rseqnum	int		// number of erasure sequences
	compat	bool		// if true, use the 0.9 file format
	rndmz	bool		// if true, randomize the data

	c1	*l1.Codec
	ec	reedsolomon.Encoder

}

// Data extent
// Describes a sequential range of data recovered
type DataExtent struct {
	Offset		uint64
	Data		[]byte
	Verified	bool		// true if the data in the extent is verified
}

var Eec = errors.New("parity blocks don't match")
var crctbl = crc64.MakeTable(crc64.ECMA)

// Creates a new L2 codec
// Parameters:
//	p5	5'-end primer
//	p3	3'-end primer
//	dblknum	number of data blocks in an oligo
//	mdsz	size of a metadata block in an oligo
//	mdcsum	number of metadata blocks for checksum
//	dseqnum	number of data oligos in the erasure block
//	rseqnum	number of erasure oligos in the erasure block
//
// Additional SetMetadataChecksum and SetDataChecksum functions
// can be called to change the behavior of the L1 codec
func NewCodec(p5, p3 oligo.Oligo, dblknum, mdsz, mdcsum, dseqnum, rseqnum int) (c *Codec, err error) {
	c = new(Codec)
	c.p5 = p5
	c.p3 = p3
	c.dblknum = dblknum
	c.dseqnum = dseqnum
	c.rseqnum = rseqnum
	c.c1, err = l1.NewCodec(dblknum, mdsz, mdcsum, criteria.H4G2)
	if err != nil {
		c = nil
		return
	}

	c.ec, err = reedsolomon.New(dseqnum, rseqnum)
	if err != nil {
		c = nil
		return
	}

	return
}

// See the description of the appropriate function in the L1 code
func (c *Codec) SetMetadataChecksum(cs int) error {
	return c.c1.SetMetadataChecksum(cs)
}

// See the description of the appropriate function in the L1 code
func (c *Codec) SetDataChecksum(cs int) error {
	return c.c1.SetDataChecksum(cs)
}

func (c *Codec) SetCompat(cpt bool) {
	c.compat = cpt
}

func (c *Codec) SetRandomize(rndmz bool) {
	c.rndmz = rndmz
}

func (c *Codec) MaxAddr() uint64 {
	return c.c1.MaxAddr()
}

// Encodes logical data into a collection of oligos.
// The addr parameter specifies the starting address for the data
// The data parameter points to the data to encode.
// The function returns the next available address as well as an
// array of oligos that encode the data.
// If the data is not aligned, it is padded with random values at
// the end.
func (c *Codec) Encode(addr uint64, data []byte) (nextaddr uint64, oligos []oligo.Oligo, err error) {
	blknum := c.c1.BlockNum()
	blksz := c.c1.BlockSize()

	if len(data) == 0 {
		err = fmt.Errorf("can't encode empty array")
		return
	}

	// first add the superblocks
	nd, super := c.addSupers(data)

	// then pad the data at the back so it's multiple of the data per erasure group
	egsz := ecGroupDataSize(blksz, blknum, c.dseqnum)
	if (len(nd) + len(super))%egsz != 0 {
		n := egsz - ((len(nd) + len(super)) % egsz)
		for i := 0; i < n; i++ {
			nd = append(nd, byte(rand.Int31n(256)))
		}
	}

	// repeat the starting super at the end
	nd = append(nd, super...)
	fmt.Fprintf(os.Stderr, "original size: %d bytes, new size %d bytes, erasure groups size %d\n", len(data), len(nd), egsz)
	for len(nd) > 0 {
		var dblks [][][]byte

		d := nd[0:egsz]
		if len(d) != egsz {
			panic("internal error")
		}

		dblks, err = ecGroupEncode(blksz, blknum, c.dseqnum, c.rseqnum, c.ec, d)
		if err != nil {
			return
		}

		for i, rblk := range dblks {
			var o oligo.Oligo

			a := addr + uint64(i)
			e := false
			if i >= c.dseqnum {
				a -= uint64(c.dseqnum)
				e = true
			}

			o, err = c.c1.Encode(c.p5, c.p3, a, e, rblk)
			if err != nil {
				return
			}

			oligos = append(oligos, o)
		}

		addr += uint64(c.dseqnum)
		nd = nd[egsz:]
	}

	nextaddr = addr
	return
}

func (c *Codec) addSupers(data []byte) (nd []byte, super []byte) {
	datasz := uint64(len(data))

	if c.rndmz {
		rnd := rand.New(rand.NewSource(int64(datasz)))
		for i := 0; i < len(data); i++ {
			r := byte(rnd.Int31n(256))
			data[i] ^= r
		}

	}

	// start with the superblock
	super = l0.Pint64(datasz, super)			// "file" size
	s := sha1.Sum(data)
	super = append(super, s[:]...)				// SHA1 sum for the whole "file"
	crc := crc64.Checksum(super, crctbl)
	super = l0.Pint64(crc, super)				// CRC64 of the superblock

	nd = append(nd, super...)
	for len(data) > 0 {
		sz := superChunkSize
		if sz > len(data) {
			sz = len(data)
		}

		// append the actual data
		nd = append(nd, data[0:sz]...)

		// append the intermediate superblock
		p := len(nd)
		nd = l0.Pint64(datasz, nd)				// "file" size
		s = sha1.Sum(data[0:sz])
		nd = append(nd, s[:]...)				// SHA1 sum for the data chunk
		crc = crc64.Checksum(nd[p:], crctbl)
		nd = l0.Pint64(crc, nd)					// CRC64 of the superblock

		data = data[sz:]
	}

	return
}

// Decodes oligos with addresses from start to end.
// The oligos array may contain extra oligo sequences that are not used.
// Return all data that we recovered in data extents
func (c *Codec) Decode(start, end uint64, oligos []oligo.Oligo) (data []DataExtent) {
	// spin up goroutines to decode
	ch := make(chan oligo.Oligo)
	f := newFile(c.dseqnum + c.rseqnum, c.c1.BlockNum(), c.c1.BlockSize(), c.rseqnum, c.ec, c.compat, c.rndmz)
	nprocs := runtime.NumCPU()
	for i := 0; i < nprocs; i++ {
		go func() {
			blknum := c.c1.BlockNum()
			dblks := make([]Blk, blknum)
			for {
//				fmt.Fprintf(os.Stderr, "waiting\n")
				ol := <- ch
				if ol == nil {
					break
				}

				addr, ef, data, err := c.c1.Decode(c.p5, c.p3, ol, 1)
				if err != nil {
//					fmt.Printf("Error: %v\n", err)
					continue
				}

				if addr < start || addr > end {
					continue
				}

				for i := 0; i < len(dblks); i++ {
					dblks[i] = Blk(data[i])
				}

				f.add(addr, ef, dblks)
			}
		}()
	}

	// feed the oligos in the order we got them
	for i, ol := range oligos {
//		fmt.Fprintf(os.Stderr, "sending %v\n", ol)
		ch <- ol
		if i != 0 && i%100000==0 {
			if f.sync() {
				// we got the whole file, no need to continue
				break
			}
		}
	}

	// shut down the processing goroutines
	for i := 0; i < nprocs; i++ {
		ch <- nil
	}

	data = f.close()
	fmt.Fprintf(os.Stderr, "%d extents\n", len(data))
	return
}

/*
func (c *Codec) EncodeECG(addr uint64, data []byte) (ols []oligo.Oligo, err error) {
	// copy/paste from Encode
	blksz := c.c1.BlockSize()
	blknum := c.c1.BlockNum()

	if len(data) != blksz*blknum*c.dseqnum {
		err = fmt.Errorf("invalid data size: %d expecting %d", len(data), blksz*blknum*c.dseqnum)
		return
	}

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
			ols = nil
			return
		}

		ols = append(ols, o)
	}

	// encode the erasure oligos for an erasure group
	for i := 0; i < c.rseqnum; i++ {
		var o oligo.Oligo

		o, err = c.c1.Encode(c.p5, c.p3, addr + uint64(i), true, egrp[c.dseqnum + i])
		if err != nil {
			ols = nil
			return
		}

		ols = append(ols, o)
	}

	return
}

func (c *Codec) DecodeECG(dfclty int, ols []oligo.Oligo) (data []DataExtent, failednum int) {
	doligos := make(map[uint64] [][][]byte)	// data oligos (list per address)
	eoligos := make(map[uint64] [][][]byte)	// erasure oligos (list per address)

	l, failed := c.decodeOligos(0, math.MaxUint64, ols, doligos, eoligos, dfclty)
	data = c.recoverData(0, l + 1, doligos, eoligos)
	failednum = len(failed)
	return
}

func (c *Codec) printECGroup(start, end uint64) {
	odlen := uint64(c.dblknum * 4)

	s := start / odlen
	e := end / odlen
	if end%odlen != 0 {
		e++
	}

	fmt.Printf("*** %d %d\n", start, end)
	s = s - (s % uint64(c.dseqnum))
	for s < e {
		fmt.Printf("---\n")
		for i := 0; i < c.dseqnum; i++ {
			fmt.Printf("%07d\n", s)
			s++
		}
	}
}

*/
