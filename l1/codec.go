package l1

import (
_	"math/bits"
	"errors"
	"fmt"
	"math"
_	"os"
	"adscodex/oligo"
_	"adscodex/oligo/long"
	"adscodex/criteria"
	"adscodex/l0"
	"github.com/klauspost/reedsolomon"
	"github.com/snksoft/crc"
)

const (
	PrimerErrors = 8	// how many errors still match the primer
)

const (
	// metadata checksums
	CSumRS	= 0
	CSumCRC	= 1
	CSumNone = 2
)

// Level 1 codec
type Codec struct {
	blknum		int	// number of data blocks
	blksz		int	// length of the data block (in nts)
	blkmindist	int	// minimum distance for data blocks
	mdnum		int	// number of metadata blocks (including checksum blocks)
	mdsz		int	// length of the metadata block (in nts)
	mdcnum		int	// number of checksum metadata blocks
	mdmindist	int	// minimum distance for metadata blocks
	crit		criteria.Criteria
	prefix		oligo.Oligo
	suffix		oligo.Oligo
	maxtime		int64

	// optional settings with defaults
	mdcsum		int	// md blocks checksum (CSumRS or CSumCRC, default CSumRS)

	dtbl		*l0.LookupTable
	mtbl		*l0.LookupTable
	grp		*l0.Group

	olen		int	// oligo length, not including the primers
	ec		reedsolomon.Encoder
	crc		*crc.Table

	datasz		int	// number of bytes that can be encoded in an oligo
	dbits		int	// number of bits that can be encoded in a single data block
	cmaxval		uint64	// maximum value that can be stored as metadata
	cbits		int	// number of bits encoded in the metadata
}

type Entry struct {
	Addr	uint64
	EcFlag	bool
	Dist	int
	Data	[]byte
	Count	int		// not set by the codec, provided externally
}

var Eprimer = errors.New("primer mismatch")
var Emetadata = errors.New("can't recover metadata")

var crcParams = []crc.Parameters {
	3: crc.Parameters{ Width: 3, Polynomial: 0x3, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x7 },		// CRC-3/GSM
	4: crc.Parameters{ Width: 4, Polynomial: 0x3, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x0 },		// CRC-4/G-704
	5: crc.Parameters{ Width: 5, Polynomial: 0x5, ReflectIn: false, ReflectOut: false, Init: 0x9, FinalXor: 0x0 },		// CRC-5-EPC
	7: crc.Parameters{ Width: 7, Polynomial: 0x65, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-7F/5
	8: crc.Parameters{ Width: 8, Polynomial: 0xa7, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x0 },		// CRC-8/BLUETOOTH
	9: crc.Parameters{ Width: 9, Polynomial: 0x79, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },		// CRC-9F/6.2
	10: crc.Parameters{ Width: 10, Polynomial: 0x3d9, ReflectIn: false, ReflectOut: false, Init: 0x3ff, FinalXor: 0x0 },	// CRC-10/CDMA2000
	11: crc.Parameters{ Width: 11, Polynomial: 0x1eb, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },	// CRC-11F/8
	13: crc.Parameters{ Width: 13, Polynomial: 0x16f, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },	// CRC-13F/8.2

	15: crc.Parameters{ Width: 15, Polynomial: 0x4599, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			// CRC-15/CAN
	16: crc.Parameters{ Width: 16, Polynomial: 0x8005, ReflectIn: true, ReflectOut: true, Init: 0x0, FinalXor: 0x0 },			// CRC-16/ARC
	17: crc.Parameters{ Width: 17, Polynomial: 0x1685b, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			// CRC-17
	19: crc.Parameters{ Width: 19, Polynomial: 0x23af3, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			// 
	22: crc.Parameters{ Width: 22, Polynomial: 0x5781eb, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			// CRC-23K/6
	23: crc.Parameters{ Width: 23, Polynomial: 0x16f3a3, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			// 
	27: crc.Parameters{ Width: 27, Polynomial: 0x4b7aa27, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			//
	30: crc.Parameters{ Width: 30, Polynomial: 0x2030b9c7, ReflectIn: false, ReflectOut: false, Init: 0x3fffffff, FinalXor: 0x3fffffff },	// CRC-30/CDMA
	37: crc.Parameters{ Width: 37, Polynomial: 0x41, ReflectIn: false, ReflectOut: false, Init: 0x0, FinalXor: 0x0 },			//
}

func NewCodec(prefix, suffix oligo.Oligo, blknum, blksz, blkmindist, mdnum, mdsz, mdcnum, mdmindist int, crit criteria.Criteria, maxtime int64) (c *Codec, err error) {
	c = new(Codec)
	c.prefix = prefix
	c.suffix = suffix
	c.blknum = blknum
	c.blksz = blksz
	c.blkmindist = blkmindist
	c.mdnum = mdnum
	c.mdsz = mdsz
	c.mdcnum = mdcnum
	c.mdmindist = mdmindist
	c.crit = crit
	c.mdcsum = CSumCRC
	c.maxtime = maxtime

	// add a metadata block at the end of the oligo if required
	if c.mdnum - c.mdcnum < 1 || (c.mdnum - c.mdcnum) < c.mdcnum {
		c.mdnum++
	}

	// get the lookup tables for data blocks
	if c.dtbl, err = l0.LoadLookupTable(l0.LookupTableFilename(crit, blksz, blkmindist)); err != nil {
		return nil, fmt.Errorf("error while loading encoding table: %v\n", err)
	}

	// get the lookup tables for the metadata blocks
	if c.mtbl, err = l0.LoadLookupTable(l0.LookupTableFilename(crit, mdsz, mdmindist)); err != nil {
		return nil, fmt.Errorf("error while loading decoding table: %v\n", err)
	}

	if err := c.updateChecksums(); err != nil {
		return nil, err
	}

//	fmt.Printf("dtbl %p mtbl %p\n", c.dtbl, c.mtbl)
	var lts []*l0.LookupTable
	n := c.blknum
	if n < c.mdnum {
		n = c.mdnum
	}

	for i := 0; i < n; i++ {
		if i < c.blknum {
			lts = append(lts, c.dtbl)
		}

		if i < c.mdnum {
			lts = append(lts, c.mtbl)
		}
	}

	c.grp, err = l0.NewGroup(prefix, lts, maxtime)	// FIXME make it easier to specify the steps4pos
	if err != nil {
		return nil, err
	}

	// how many bits can we encode in the oligo?
	dbits := c.blknum * int(math.Log2(float64(c.dtbl.MaxVal())))

	// how many bits can we encode per data block?
	c.dbits = dbits / c.blknum

	// we want each oligo to encode 8-bit aligned data
	c.datasz = (c.dbits * c.blknum) / 8

	c.olen = c.blksz * c.blknum + c.mdsz * c.mdnum + prefix.Len() + suffix.Len()
	return
}

// Change which checksum algorithm is used to protect the metadata blocks
func (c *Codec) SetMetadataChecksum(cs int) error {
	c.mdcsum = cs
	return c.updateChecksums()
}

func (c *Codec) updateChecksums() (err error) {
	mdmaxval := c.mtbl.MaxVal()
	cmaxval := math.Pow(float64(mdmaxval), float64(c.mdnum - c.mdcnum))
	c.cbits = int(math.Floor(math.Log2(float64(mdmaxval))))
	c.cmaxval = uint64(cmaxval)
//	fmt.Printf("cmaxval: %v\n", cmaxval)

	if c.mdcnum == 0 {
		c.mdcsum = CSumNone
	}

	switch c.mdcsum {
	default:
		err = errors.New("invalid metadata checksum")
		return

	case CSumRS:
		err = errors.New("RS is currently not supported")
		return
/*
		if mdmaxval > 255 {
			err = errors.New("RS currently supports no more than 8 bits for metadata block")
			return
		}

		c.crc = nil
		if c.ec == nil {
			c.ec, err = reedsolomon.New(c.mdnum - c.mdrsnum, c.mdrsnum)
		}
*/

	case CSumCRC:
		c.ec = nil
		if len(crcParams) <= c.cbits || crcParams[c.cbits].Width == 0 {
			err = fmt.Errorf("unsupported CRC length: %d", c.cbits)
			return
		}

		c.crc = crc.NewTable(&crcParams[c.cbits])

	case CSumNone:
	}

	return
}

// number of blocks per oligo
func (c *Codec) BlockNum() int {
	// FIXME: L2 actually needs the number of bytes right now
	return c.datasz
//	return c.blknum
}

// number of bits per data block
func (c *Codec) BlockBits() int {
	return c.dbits
}

// length of the data saved per oligo (in bytes)
func (c *Codec) DataLen() int {
	return c.datasz
}

func (c *Codec) BlockSize() int {
	return 1
}

func (c *Codec) OligoLen() int {
	return c.olen
}

// maximum address that the codec can encode
func (c *Codec) MaxAddr() uint64 {
	return uint64(c.cmaxval / 2)
}

// Encode data into a an oligo
// The ef parameter specifies whether the oligo is an erasure oligo (i.e. provides some erasure data 
// instead of data data).
func (c *Codec) Encode(address uint64, ef bool, data []byte) (ret oligo.Oligo, err error) {
	var mdb, db []uint64
	var ol oligo.Oligo

	if len(data) != c.datasz {
		return nil, fmt.Errorf("L1: invalid data size %d:%d", len(data), c.datasz)
	}

	mdb, err = c.calculateMdBlocks(address, ef)
	if err != nil {
		return nil, err
	}

	db, err = c.calculateDataBlocks(data)
	if err != nil {
		return nil, err
	}

	var vals []int
	n := c.blknum
	if n < c.mdnum {
		n = c.mdnum
	}

	for i := 0; i < n; i++ {
		if i < c.blknum {
			vals = append(vals, int(db[i]))
		}

		if i < c.mdnum {
			vals = append(vals, int(mdb[i]))
		}
	}

	ol, err = c.grp.Encode(vals)
	if err != nil {
		ret = nil
		return
	}

//	fmt.Printf("Encode: %v %v %v %v dbits %d \n", vals, ol, db, mdb, c.dbits)
	// append the prefix
	ret = c.prefix.Clone()
	ret.Append(ol)

	// append the suffix
	// FIXME: we don't apply the criteria when appending p3,
	// so theoretically we can have homopolymers etc.
	ret.Append(c.suffix)
	return
}

func (c *Codec) calculateDataBlocks(data []byte) (ret []uint64, err error) {
	var val uint64

//	fmt.Printf("calcDataBlocks %x %v\n", data, data)
	dmask := uint64(1) << c.dbits - 1
	bits := 0
	n := len(data) - 1
	for len(ret) < c.blknum {
		if n < 0 || bits > c.dbits {
			ret = append(ret, val & dmask)
			val >>= c.dbits
			bits -= c.dbits
//			fmt.Printf("\tval %x bits %d append %x\n", val, bits, ret[len(ret) - 1])
		} else {
			val = (uint64(data[n]) << bits) | val // (val << 8) | uint64(data[n])
			n--
			bits += 8
//			fmt.Printf(">>> val %x n %d bits %d\n", val, n, bits)
		}
	}

	return
}

// calculate the metadata blocks based on the metadata
func (c *Codec) calculateMdBlocks(address uint64, ef bool) ([]uint64, error) {
	maxaddr := c.MaxAddr()
	if address > maxaddr {
		return nil, fmt.Errorf("address too big %d:%d", address, maxaddr)
	}

	// calculate the metadata value
	if ef {
		address += maxaddr
	}

	// split the metadata into md blocks
	mdnum := uint64(c.mdnum - c.mdcnum)
	mdmaxval := uint64(c.mtbl.MaxVal())
	mdb := make([]uint64, c.mdnum)
	for i := int(mdnum - 1); i >= 0; i-- {
		mdb[i] = address % mdmaxval
		address /= mdmaxval
	}

	if address != 0 {
		panic("Internal error: address not zero at the end")
	}

	switch (c.mdcsum) {
	default:
		panic("unsupported md checksum")

/*
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

		for i := c.mdnum - c.rsnum; i < c.mdnum; i++ {
			mdb[i] = uint64(mdshard[i][0])
		}
*/

	case CSumCRC:
		cval := c.crc.InitCrc()
//		fmt.Fprintf(os.Stderr, "+ %v\n", mdb)
		for i := uint64(0); i < mdnum; i++ {
			// TODO: works up to 16 bit metadata blocks
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdb[i]), byte(mdb[i]>>8)})
//			fmt.Fprintf(os.Stderr, "\t%v: %v\n", []byte { byte(mdb[i]), byte(mdb[i]>>8)}, cval)
		}
		cval = c.crc.CRC(cval)
//		cv := cval
		for i := c.mdnum - c.mdcnum; i < c.mdnum; i++ {
			mdb[i] = cval % mdmaxval
			cval /= mdmaxval
		}

//		fmt.Fprintf(os.Stderr, "\tmdblks %v crc %d rem %d\n", mdb, cv, cval)

	case CSumNone:
	}

	return mdb, nil
}

// Decodes an oligo into the metadata and data it contains
// If the recover parameter is true, try harder to correct the metadata
// Returns a byte array for each data block that was recovered
// (i.e. the parity for the block was correct)
func (c *Codec) Decode(ol oligo.Oligo) (address uint64, ef bool, data []byte, errdist int, err error) {
	var vals []int
	var db, md []uint64

	col := c.cutPrimers(ol)
	if col == nil {
		err = fmt.Errorf("primers not found: %v\n", ol)
		return
	}

	vals, errdist, err = c.grp.Decode(c.prefix, col)
	if err != nil {
		return
	}

	// split data and metadata blocks
	for i := 0; i < c.blknum + c.mdnum;  {
		if len(db) < c.blknum {
			db = append(db, uint64(vals[i]))
			i++
		}

		if len(md) < c.mdnum {
			md = append(md, uint64(vals[i]))
			i++
		}
	}

//	fmt.Printf("Decode: %v %v : %v %v\n", vals, col, db, md)

	address, ef, err = c.recoverMetadata(md)
	if err != nil {
		return
	}

	data = c.recoverData(db)
	return
}

func (c *Codec) cutPrimers(ol oligo.Oligo) (ret oligo.Oligo) {
	// First cut the primers
	pos5, len5 := oligo.Find(ol, c.prefix, PrimerErrors)
	if pos5 != 0 {
		return
	}

	pos3, _/*len3*/ := oligo.Find(ol, c.suffix, PrimerErrors)
	if pos3 < 0 /*|| pos3+len3 != ol.Len()*/ {
		return
	}

	ret = ol.Slice(pos5+len5, pos3)
//	prefix = p5.Slice(p5.Len() - 4, p5.Len())

	return
}

func (c *Codec) recoverMetadata(mb []uint64) (address uint64, ef bool, err error) {
	var md uint64

	if ok, e := c.checkMDBlocks(mb); !ok {
		if e != nil {
			err = e
		} else {
			err = fmt.Errorf("metadata checksum error")
		}

		return
	}

	mdmaxval := uint64(c.mtbl.MaxVal())
	for i := 0; i < c.mdnum - c.mdcnum; i++ {
		nmd := md * mdmaxval + mb[i]
		if nmd < md {
			// overflow, panic
			panic("metadata overflow")
		}

		md = nmd
	}

	maxaddr := c.MaxAddr()
	if md >= maxaddr {
		ef = true
		md -= maxaddr
	}

	address = md
	return
}

func (c *Codec) checkMDBlocks(mdblks []uint64) (ok bool, err error) {
	switch c.mdcsum {
	default:
		panic("invalid metadata checksum type")
/*
	case CSumRS:
		mdshards := make([][]byte, len(mdblks))
		for i, v := range mdblks {
			mdshards[i] = []byte { byte(v) }
		}

		ok, err = c.ec.Verify(mdshards)
		if err != nil {
			ok = false
		}
*/

	case CSumCRC:
		cval := c.crc.InitCrc()
//		fmt.Fprintf(os.Stderr, "- mdblks %v\n", mdblks)
		for i := 0; i < c.mdnum - c.mdcnum; i++ {
			cval = c.crc.UpdateCrc(cval, []byte { byte(mdblks[i]), byte(mdblks[i]>>8) })
//			fmt.Fprintf(os.Stderr, "\t%v: %v\n", []byte { byte(mdblks[i]), byte(mdblks[i]>>8)}, cval)
		}
		cval = c.crc.CRC(cval)

		cval2 := uint64(0)
		mval := uint64(c.mtbl.MaxVal())
		for i := c.mdnum - 1; i >= c.mdnum - c.mdcnum; i-- {
			cval2 = (cval2 * mval) + mdblks[i]
		}

		ok = cval == cval2

	case CSumNone:
		ok = true
	}

	return
}

func (c *Codec) recoverData(db []uint64) (ret []byte) {
	var val uint64

	ret = make([]byte, c.datasz)

//	fmt.Printf("recoverData %v\n", db)
	bits := 0
	n := 0 // len(db) - 1
	i := c.datasz - 1
	for i >= 0 /*len(ret) != c.datasz*/ {
		if bits > c.dbits {
			d := byte(val)

			ret[i] = d // ret = append(ret, d)
			i--
			val >>= 8
			bits -= 8
//			fmt.Printf("\tval %x bits %d append %x\n", val, bits, d)
		} else {
			if n < len(db) {
				val = db[n]<<bits | val // (val << c.dbits) | db[n]
				n++
			}

			bits += c.dbits
//			fmt.Printf("<<< val %x n %d bits %d\n", val, n, bits)
		}
	}

	return
}
