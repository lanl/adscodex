package l1

import (
_	"math/bits"
	"errors"
	"fmt"
_	"os"
	"adscodex/oligo"
	"adscodex/oligo/long"
_	"adscodex/criteria"
	"adscodex/l0"
	"github.com/klauspost/reedsolomon"
	"github.com/snksoft/crc"
)

const (
	PrimerErrors = 8	// how many errors still match the primer
)

const (
	dblkSize = 17		// data block size
	dblkMaxval = 2<<33
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
	datanum	int	// number of data blocks/bytes
	mdnum	int	// number of metadata blocks/bytes
	cmdnum	int	// number of checksum blocks

	// optional settings with defaults
	mdcsum	int	// md blocks checksum (CSumRS or CSumCRC, default CSumRS)

	c0	*l0.Codec
	olen	int	// oligo length, not including the primers
	ec	reedsolomon.Encoder
	crc	*crc.Table
}


var Eprimer = errors.New("primer mismatch")
var Emetadata = errors.New("can't recover metadata")

var crcParams = []crc.Parameters {
	 8: crc.Parameters{Width: 8, Polynomial: 0x2F, Init: 0xFF, ReflectIn: false, ReflectOut: false, FinalXor: 0xFF},
	16: crc.Parameters{Width: 16, Polynomial: 0x8005, Init: 0x0000, ReflectIn: true, ReflectOut: true, FinalXor: 0x0},
	24: crc.Parameters{Width: 24, Polynomial: 0x00065B, Init: 0x555555, ReflectIn: true, ReflectOut: true, FinalXor: 0x0},
	32: crc.Parameters{Width: 32, Polynomial: 0x04C11DB7, Init: 0xFFFFFFFF, ReflectIn: true, ReflectOut: true, FinalXor: 0xFFFFFFFF},
	40: crc.Parameters{Width: 40, Polynomial: 0x0004820009, Init: 0x0000000000, ReflectIn: false, ReflectOut: false, FinalXor: 0xffffffffff },
	64: crc.Parameters{Width: 64, Polynomial: 0x000000000000001B, Init: 0xFFFFFFFFFFFFFFFF, ReflectIn: true, ReflectOut: true, FinalXor: 0xFFFFFFFFFFFFFFFF},
}

func NewCodec(datanum, mdnum, cmdnum int, c0 *l0.Codec) (c *Codec, err error) {
	c = new(Codec)
	c.datanum = datanum
	c.cmdnum = cmdnum
	c.mdnum = mdnum
	c.c0 = c0
	c.mdcsum = CSumCRC

	c.olen = (c.mdnum + c.datanum) * c.c0.OligoLen()
	if err := c.updateChecksums(); err != nil {
		return nil, err
	}

	return
}

// Change which checksum algorithm is used to protect the metadata blocks
func (c *Codec) SetMetadataChecksum(cs int) error {
	c.mdcsum = cs
	return c.updateChecksums()
}

func (c *Codec) updateChecksums() (err error) {
	switch c.mdcsum {
	default:
		err = errors.New("invalid metadata checksum type")
		return

	case CSumRS:
		c.crc = nil
		if c.ec == nil {
			c.ec, err = reedsolomon.New(c.mdnum, c.cmdnum)
		}

	case CSumCRC:
		c.ec = nil
		n := 8 * c.cmdnum
		if len(crcParams) <= n || crcParams[n].Width == 0 {
			err = fmt.Errorf("unsupported CRC length: %d", n)
			return
		}

		c.crc = crc.NewTable(&crcParams[n])
	}

	return
}

// number of blocks per oligo
func (c *Codec) DataNum() int {
	return c.datanum
}

// number of bytes per data block
func (c *Codec) BlockSize() int {
	return 1
}

// length of the data saved per oligo (in bytes)
func (c *Codec) DataLen() int {
	return c.datanum
}

func (c *Codec) OligoLen() int {
	return c.olen
}

// maximum address that the codec can encode
func (c *Codec) MaxAddr() uint64 {
	ma := uint64(1) << (c.mdnum * 8)

	return ma / 4
}

// Encode data into a an oligo
// The p5 and p3 oligos specify the 5'-end and the 3'-end primers that start and end the oligo. At the
// moment p5 needs to be at least 4 nts long.
// The ef parameter specifies whether the oligo is an erasure oligo (i.e. provides some erasure data 
// instead of data data).
func (c *Codec) Encode(p5, p3 oligo.Oligo, address uint64, ef bool, data []byte) (o oligo.Oligo, err error) {
	o, err = c.encode(p5, p3, address, ef, false, data)
	if err != nil {
		return
	}

	if gc := oligo.GCcontent(o); gc > 0.6 {
		var o1 oligo.Oligo

		o1, err = c.encode(p5, p3, address, ef, true, data)
		if err != nil {
			return
		}

		if gc1 := oligo.GCcontent(o1); gc1 < gc {
			o = o1
		}
	}

	return
}

// The actual implementation of the encoding. 
// The sf paramter defines if the payload needs to be negated so 
// the GC content is kept low.
func (c *Codec) encode(p5, p3 oligo.Oligo, address uint64, ef, sf bool, data []byte) (o oligo.Oligo, err error) {
	var mdb []uint64

	if len(data) != c.DataLen() {
		return nil, errors.New("invalid data length")
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
		d := make([]byte, len(data))
		for i := 0; i < len(data); i++ {
			d[i] = ^data[i]
		}
		data = d

		// TODO: do something similar for metadata
	}

	// Construct the oligo
	// start with the 5'-end primer
	o, _ = long.Copy(p5)
//	s := ""
	for i, b := range data {
		var ol oligo.Oligo

		// append the data block
		prefix := o.Slice(o.Len() - c.c0.PrefixLen(), o.Len())
		ol, err = c.c0.Encode(prefix, uint64(b))
		if err != nil {
			return nil, err
		}
		o.Append(ol)
//		s += fmt.Sprintf("%d (%v|%v) ", b, prefix, ol)

		// append the metadata block (if any)
		if i < len(mdb) {
			prefix = o.Slice(o.Len() - c.c0.PrefixLen(), 0)
			ol, err = c.c0.Encode(prefix, mdb[i])
			if err != nil {
				return nil, err
			}

			o.Append(ol)
//			s += fmt.Sprintf("+%d (%v|%v) ", mdb[i], prefix, ol)
		}
	}

	// append the 3'-end primer
	// FIXME: we don't apply the criteria when appending p3,
	// so theoretically we can have homopolymers etc.
	o.Append(p3)

//	fmt.Printf("%s\n", s)
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

	mdb := make([]uint64, c.mdnum + c.cmdnum)
	for i := 0; i < c.mdnum; i++ {
		mdb[i] = address & 0xFF
		address >>= 8
	}

	switch (c.mdcsum) {
	default:
		panic("unsupported md checksum")

	case CSumRS:
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

		for i := c.mdnum; i < len(mdshard); i++ {
			mdb[i] = uint64(mdshard[i][0])
		}

	case CSumCRC:
		cval := c.crc.InitCrc()
		for i := 0; i < c.mdnum; i++ {
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdb[i]) })
		}
		cval = c.crc.CRC(cval)
		for i := c.mdnum; i < len(mdb); i++ {
			mdb[i] = cval & 0xFF
			cval >>= 8
		}
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
		for i := 0; i < c.mdnum; i++ {
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdblks[i]) })
		}
		cval = c.crc.CRC(cval)

		cval2 := uint64(0)
		for i := len(mdblks) - 1; i >= c.mdnum; i-- {
			cval2 = (cval2 << 8) | (mdblks[i] & 0xFF)
		}

		ok = cval == cval2
	}

	return
}

// Decodes an oligo into the metadata and data it contains
// If the recover parameter is true, try harder to correct the metadata
// Returns a byte array for each data block that was recovered
// (i.e. the parity for the block was correct)
func (c *Codec) Decode(p5, p3, ol oligo.Oligo, difficulty int) (address uint64, ef bool, data []byte, err error) {
	address, ef, data, err = c.decode(p5, p3, ol, difficulty)
	return
}

func (c *Codec) decode(p5, p3, ol oligo.Oligo, difficulty int) (address uint64, ef bool, data []byte, err error) {
	address, ef, data, err = c.tryDecode(p5, p3, ol, difficulty)
	return
}
