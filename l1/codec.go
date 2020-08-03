package l1

import (
	"math/bits"
	"errors"
	"fmt"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/criteria"
	"acoma/l0"
	"github.com/klauspost/reedsolomon"
	"github.com/snksoft/crc"
)

const (
	PrimerErrors = 3	// how many errors still match the primer
)

const (
	dblkSize = 17		// data block size
)

const (
	// metadata checksums
	CSumRS	= 0
	CSumCRC	= 1

	// data checksums
	CSumParity = 0		// if the number of bits that are 1 is odd, the checksum bit is one
	CSumEven = 1		// if the value is odd, the checksum bit is one
)

// Level 1 codec
type Codec struct {
	blknum	int	// number of data blocks
	rsnum	int	// number of Reed-Solomon metadata blocks
	mdsz	int	// length of the metadata block (in nts)
	crit	criteria.Criteria

	// optional settings with defaults
	mdcsum	int	// md blocks checksum (CSumRS or CSumCRC, default CSumRS)
	dtcsum	int	// data blocks checksum (CSumParity, CSumEven, ..., default CSumParity)

	olen	int	// oligo length, not including the primers
	ec	reedsolomon.Encoder
	crc	*crc.Table
}


var Eprimer = errors.New("primer mistmatch")
var Emetadata = errors.New("can't recover metadata")

var maxvals = []int {
	3: 47,
	4: 186,
	5: 733,
	6: 2889,
	7: 11388,
	8: 44891,
	9: 176955,
	10: 697537,
	12: 10838676,
	14: 168416727,
}

var crcParams = []crc.Parameters {
//	0: crc.Parameters{ Width: 3, Polynomial: 0x3, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x7 },		// CRC-3/GSM
//	0: crc.Parameters{ Width: 4, Polynomial: 0x3, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x0 },		// CRC-4/G-704
	3: crc.Parameters{ Width: 5, Polynomial: 0x5, ReflectIn: false, ReflectOut: false, Init: 0x9, FinalXor: 0x0 },		// CRC-5-EPC
	4: crc.Parameters{ Width: 7, Polynomial: 0x65, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-7F/5
	5: crc.Parameters{ Width: 9, Polynomial: 0x79, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-9F/6.2
	6: crc.Parameters{ Width: 11, Polynomial: 0x1eb, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },	// CRC-11F/8
	7: crc.Parameters{ Width: 13, Polynomial: 0x16f, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },	// CRC-13F/8.2

	8: crc.Parameters{ Width: 15, Polynomial: 0x4599, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-15/CAN
	9: crc.Parameters{ Width: 17, Polynomial: 0x1685b, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-17
	10: crc.Parameters{ Width: 19, Polynomial: 0x23af3, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// 
	12: crc.Parameters{ Width: 23, Polynomial: 0x16f3a3, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// 
	14: crc.Parameters{ Width: 27, Polynomial: 0x4b7aa27, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		//
	18: crc.Parameters{ Width: 37, Polynomial: 0x41, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		//
}

func NewCodec(blknum, mdsz, rsnum int, crit criteria.Criteria) *Codec {
	c := new(Codec)
	c.blknum = blknum
	c.rsnum = rsnum
	c.mdsz = mdsz
	c.crit = crit
	c.mdcsum = CSumCRC

	// TODO: make it work with longer metadata blocks
	if maxvals[mdsz * rsnum] == 0 {
		fmt.Printf("invalid mdsz")
		return nil
	}

	if err := c.updateChecksums(); err != nil {
		panic(err)
		return nil
	}

	return c
}

// Change which checksum algorithm is used to protect the metadata blocks
func (c *Codec) SetMetadataChecksum(cs int) error {
	c.mdcsum = cs
	return c.updateChecksums()
}

// Change which checksum algorithm is used to protect the data blocks
func (c *Codec) SetDataChecksum(cs int) error {
	if cs != CSumParity && cs != CSumEven {
		return fmt.Errorf("invalid data checksum type: %d", cs)
	}

	c.dtcsum = cs
	return nil
}

func (c *Codec) updateChecksums() (err error) {
	switch c.mdcsum {
	default:
		err = errors.New("invalid metadata checksum")
		return

	case CSumRS:
		if c.mdsz > 4 {
			err = errors.New("RS currently supports no more than 4nt of length")
			return
		}

		c.crc = nil
		if c.ec == nil {
			c.ec, err = reedsolomon.New(c.blknum - c.rsnum, c.rsnum)
		}

		c.olen = c.blknum * dblkSize +		// data blocks
			c.mdsz*(c.blknum - c.rsnum) +	// metadata blocks
			5*c.rsnum		  	// metadata erasure blocks (they have to be able to store a byte)

	case CSumCRC:
		c.ec = nil
		n := c.mdsz * c.rsnum
		if len(crcParams) <= n || crcParams[n].Width == 0 {
			err = fmt.Errorf("unsupported CRC length: %d", n)
			return
		}

		c.crc = crc.NewTable(&crcParams[n])
		c.olen = c.blknum * dblkSize +		// data blocks
			c.blknum * c.mdsz		// metadata blocks (including erasure)
	}

	return
}

// number of blocks per oligo
func (c *Codec) BlockNum() int {
	return c.blknum
}

// number of bytes per data block
func (c *Codec) BlockSize() int {
	return 4
}

// length of the data saved per oligo (in bytes)
func (c *Codec) DataLen() int {
	return c.blknum * 4
}

func (c *Codec) OligoLen() int {
	return c.olen
}

// maximum address that the codec can encode
func (c *Codec) MaxAddr() uint64 {
	mdnum := c.blknum - c.rsnum

	ma := uint64(1)
	maxval :=uint64( maxvals[c.mdsz])
	for i := 0; i < mdnum; i++ {
		ma *= maxval
	}

	return uint64(ma / 4)
}

func (c *Codec) mdcsumLen() (n int) {
	switch c.mdcsum {
	default:
		n = -1

	case CSumRS:
		n = 5

	case CSumCRC:
		n = c.mdsz
	}

	return
}

// Encode data into a an oligo
// The p5 and p3 oligos specify the 5'-end and the 3'-end primers that start and end the oligo. At the
// moment p5 needs to be at least 4 nts long.
// The ef parameter specifies whether the oligo is an erasure oligo (i.e. provides some erasure data 
// instead of data data).
func (c *Codec) Encode(p5, p3 oligo.Oligo, address uint64, ef bool, data [][]byte) (o oligo.Oligo, err error) {
	o, err = c.encode(p5, p3, address, ef, false, data)
	if err == nil && oligo.GCcontent(o) > 0.6 {
		var o1 oligo.Oligo

		o1, err = c.encode(p5, p3, address, ef, true, data)
		if err == nil {
			if oligo.GCcontent(o1) > 0.6 {
				// FIXME: should we just pick the one that has lower content?
				panic("both high GC content")
			}

			o = o1
		}
	}

	return
}

// The actual implementation of the encoding. 
// The sf paramter defines if the payload needs to be negated so 
// the GC content is kept low.
func (c *Codec) encode(p5, p3 oligo.Oligo, address uint64, ef, sf bool, data [][]byte) (o oligo.Oligo, err error) {
	var mdb []uint64
	var b oligo.Oligo

	if len(data) != c.BlockNum() {
		return nil, errors.New("invalid block number")
	}

	for _, blk := range data {
		if len(blk) != c.BlockSize() {
			return nil, errors.New("invalid data size")
		}
	}

	// TODO: should we make it work without primers?
	if p5.Len() < 4 {
		return nil, errors.New("5'-end primer must be at least four nt long")
	}

	mdb, err = c.calculateMdBlocks(address, ef, sf)
	if err != nil {
		return nil, err
	}

	// negate the values if sf is true
	if sf {
		d := make([][]byte, len(data))
		for i := 0; i < len(data); i++ {
			d[i] = make([]byte, len(data[i]))
			for j := 0; j < len(data[i]); j++ {
				d[i][j] = ^data[i][j]
			}
		}
		data = d

		// TODO: do something similar for metadata
	}

	// Construct the oligo
	// start with the 5'-end primer
	o, _ = long.Copy(p5)
	for i := 0; i < c.blknum; i++ {
		buf := data[i]

		// combine the data bytes into uint64
		v := uint64(buf[0]) | (uint64(buf[1]) << 8) | (uint64(buf[2]) << 16) |
                        (uint64(buf[3]) << 24)

		// calculate parity
		var parity uint64
		switch c.dtcsum {
		default:
			panic("unknown data checksum type")

		case CSumParity:
			parity = uint64(bits.OnesCount64(v)) % 2

		case CSumEven:
			parity = v % 2
		}

		v = (v<<1) + parity

		// append the data block
		prefix := o.Slice(o.Len() - 4, o.Len())
		b, err = l0.Encode(prefix, v, dblkSize, c.crit)
		if err != nil {
			return nil, err
		}
		o.Append(b)

		// append the metadata block
		prefix = o.Slice(o.Len() - 4, 0)

		// FIXME: the RS implementation that we are using works on bytes
		// So the erasure metadata blocks need to be 8 bits long, no matter
		// what the size of the metadata blocks is. 
		// We should find a variable-bit-length RS implementation for the 
		// metadata
		sz := c.mdsz
		if i >= c.blknum - c.rsnum {
			sz = c.mdcsumLen()
		}

		b, err = l0.Encode(prefix, mdb[i], sz, c.crit)
		if err != nil {
			return nil, err
		}

		o.Append(b)
	}

	// append the 3'-end primer
	// FIXME: we don't apply the criteria when appending p3,
	// so theoretically we can have homopolymers etc.
	o.Append(p3)

	return o, nil
}

// calculate the metadata blocks based on the metadata
func (c *Codec) calculateMdBlocks(address uint64, ef, sf bool) ([]uint64, error) {
	maxaddr := c.MaxAddr()
	if address > maxaddr {
		return nil, errors.New("address too big")
	}

	// calculate the metadata value
	if sf {
		address += maxaddr * 2
	}

	if ef {
		address += maxaddr
	}

	// split the metadata into md blocks
	mdnum := uint64(c.blknum - c.rsnum)
	mdlen := uint64(maxvals[c.mdsz])
	mdb := make([]uint64, mdnum + uint64(c.rsnum))
	for i := int(mdnum - 1); i >= 0; i-- {
		mdb[i] = address % mdlen
		address /= mdlen
	}

	if address != 0 {
		panic("Internal error: address not zero at the end")
	}

	switch (c.mdcsum) {
	default:
		panic("unsupported md checksum")

	case CSumRS:
		if c.mdsz * 2 > 8 {
			panic("metadata block too big (FIXME)")
		}

		// calculate metadata erasure blocks
		// first we need to convert the metadata blocks to arrays of bytes
		mdshard := make([][]byte, len(mdb))
		for i := 0; i < len(mdshard); i++ {
			mdshard[i] = make([]byte, 1)
			mdshard[i][0] = byte(mdb[i])
		}

		err := c.ec.Encode(mdshard)
		if err != nil {
			return nil, err
		}

		for i := c.blknum - c.rsnum; i < c.blknum; i++ {
			mdb[i] = uint64(mdshard[i][0])
		}

	case CSumCRC:
		cval := c.crc.InitCrc()
//		fmt.Printf("+ %v\n", mdb)
		for i := uint64(0); i < mdnum; i++ {
			// TODO: works up to 16 bit metadata blocks
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdb[i]), byte(mdb[i]>>8)})
//			fmt.Printf("\t%v: %v\n", []byte { byte(mdb[i]), byte(mdb[i]>>8)}, cval)
		}
		cval = c.crc.CRC(cval)
//		cv := cval
		mval := uint64(maxvals[c.mdsz])
		for i := c.blknum - c.rsnum; i < c.blknum; i++ {
			mdb[i] = cval % mval
			cval /= mval
		}

//		fmt.Printf("\tmdblks %v crc %d\n", mdb, cv)
	}

	return mdb, nil
}

func (c *Codec) checkMDBlocks(mdblks []uint64) (ok bool, err error) {
	switch c.mdcsum {
	default:
		panic("invalid metadata checksum type")

	case CSumRS:
		mdshards := make([][]byte, len(mdblks))
		for i, v := range mdblks {
			mdshards[i] = []byte { byte(v) }
		}

		ok, err = c.ec.Verify(mdshards)
		if err != nil {
			ok = false
		}

	case CSumCRC:
		cval := c.crc.InitCrc()
//		fmt.Printf("- mdblks %v\n", mdblks)
		for i := 0; i < c.blknum - c.rsnum; i++ {
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdblks[i]), byte(mdblks[i]>>8) })
//			fmt.Printf("\t%v: %v\n", []byte { byte(mdblks[i]), byte(mdblks[i]>>8)}, cval)
		}
		cval = c.crc.CRC(cval)

		cval2 := uint64(0)
		mval := uint64(maxvals[c.mdsz])
		for i := c.blknum - 1; i >= c.blknum - c.rsnum; i-- {
			cval2 = (cval2 * mval) + mdblks[i]
		}

//		fmt.Printf("\tmdblks %v crc %d calculated crc %d\n", mdblks, cval2, cval)

		ok = cval == cval2
	}

	return
}

func (c *Codec) checkDataBlock(dblk uint64) (ok bool, data []byte) {
	pbit := int(dblk & 1)
	dblk >>= 1

	ok = false
	switch c.dtcsum {
	default:
		panic("invalid data block checksum type")

	case CSumParity:
		ok = (bits.OnesCount64(dblk) + pbit) % 2 == 0

	case CSumEven:
		ok = (dblk + uint64(pbit)) % 2  == 0
	}


	if !ok {
		return
	}

	data = make([]byte, 4)
	data[0] = byte(dblk)
	data[1] = byte(dblk >> 8)
	data[2] = byte(dblk >> 16)
	data[3] = byte(dblk >> 24)

	return
}

// Decodes an oligo into the metadata and data it contains
// If the recover parameter is true, try harder to correct the metadata
// Returns a byte array for each data block that was recovered
// (i.e. the parity for the block was correct)
func (c *Codec) Decode(p5, p3, ol oligo.Oligo, difficulty int) (address uint64, ef bool, data [][]byte, err error) {
	address, ef, data, err = c.decode(p5, p3, ol, difficulty)
	return
}

func (c *Codec) decode(p5, p3, ol oligo.Oligo, difficulty int) (address uint64, ef bool, data [][]byte, err error) {
	var sf bool
	var e error

	// TODO: fix this
	if p5.Len() < 4 || p3.Len() < 4 {
		panic("primers too short")
	}

	// First cut the primers
	pos5, len5 := oligo.Find(ol, p5, PrimerErrors)
	if pos5 != 0 {
		err = Eprimer
		return
	}

	pos3, len3 := oligo.Find(ol, p3, PrimerErrors)
	if pos3 < 0 || pos3+len3 != ol.Len() {
		err = Eprimer
		return
	}
	ol = ol.Slice(pos5+len5, pos3)
	p5suffix := p5.Slice(p5.Len() - 4, p5.Len())

	// Next, try to decode the metadata.
	mdblks := make([]uint64, c.blknum)
	mdok := true

	// collect metadata
	for i, mdpos := 0, dblkSize; i < c.blknum; i++ {
		mdsz := c.mdsz
		if i >= c.blknum - c.rsnum {
			mdsz = c.mdcsumLen()	// erasure blocks might be differeent size
		}

		mdpfx := ol.Slice(mdpos - 4, mdpos)
		mdol := ol.Slice(mdpos, mdpos + mdsz)
		if mdol.Len() != mdsz {
			// short oligo
			mdok = false
			break
		}

		mdblks[i], e = l0.Decode(mdpfx, mdol, c.crit)
		if e != nil {
			mdok = false
			break
		}

		mdpos += dblkSize + mdsz
	}

	// check if the erasure codes match
	if mdok {
		mdok, err = c.checkMDBlocks(mdblks)
	}

	if mdok {
		// Next decode the data
		dpfx := p5suffix
		for i, dpos := 0, 0; i < c.blknum; i++ {
			var v uint64
			var d []byte

			dol := ol.Slice(dpos, dpos + dblkSize)
			if dol.Len() != dblkSize {
				goto savedblk
			}

			v, e = l0.Decode(dpfx, dol, c.crit)
			if e == nil {
				var ok bool

				ok, d = c.checkDataBlock(v)
				if !ok {
					// just in case
					d = nil
				}
			}

savedblk:
			data = append(data, d)

			mdsz := c.mdsz
			if i >= c.blknum - c.rsnum {
				mdsz = c.mdcsumLen()
			}

			dpos += dblkSize + mdsz
			dpfx = ol.Slice(dpos - 4, dpos)
		}
	} else {
		// The metadata didn't compute
		if difficulty == 0 {
			err = Emetadata
			return
		}

		// Try to recover the metadata, and eventually get better at the data too
		data, mdblks, err = c.tryRecover(p5suffix, ol, difficulty)
		if err != nil {
			return
		}
	}

	// FIXME: md can be more than 64 bits
	md := uint64(0)
	maxval := uint64(maxvals[c.mdsz])
	for i := 0; i < c.blknum - c.rsnum; i++ {
		md = md * maxval + mdblks[i]
	}

	maxaddr := c.MaxAddr()
	if md >= 2*maxaddr {
		sf = true
		md -= 2*maxaddr
	}

	if md >= maxaddr {
		ef = true
		md -= maxaddr
	}

	address = md
	if sf {
		// invert the data
		for _, dblk := range data {
			for i := 0; i < len(dblk); i++ {
				dblk[i] = ^dblk[i]
			}
		}
	}

	return	
}

