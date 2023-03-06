package l1

import (
_	"math/bits"
	"errors"
	"fmt"
_	"os"
	"adscodex/oligo"
_	"adscodex/oligo/long"
	"adscodex/l0"
)

const (
	PrimerErrors = 8	// how many errors still match the primer
)

// Level 1 codec
type Codec struct {
	prefix	oligo.Oligo
	suffix	oligo.Oligo
	maxtime	int64

	// optional settings with defaults
	c0	*l0.Codec

	olen	int	// oligo length, not including the primers
	cmaxval	uint64	// maximum value that can be stored as metadata
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

func NewCodec(prefix, suffix oligo.Oligo, tblFile string,  maxtime int64) (c *Codec, err error) {
	c = new(Codec)
	c.prefix = prefix
	c.suffix = suffix
	c.maxtime = maxtime

	c.c0, err = l0.New(tblFile, maxtime)
	if err != nil {
		return nil, fmt.Errorf("error while loading encoding table: %v\n", err)
	}

	c.olen = c.c0.OligoLen()
	c.cmaxval = c.c0.MaxVal() / 256
	return
}

// number of blocks per oligo
func (c *Codec) BlockNum() int {
	return 1
}

// number of bits per data block
func (c *Codec) BlockBits() int {
	return 8
}

// length of the data saved per oligo (in bytes)
func (c *Codec) DataLen() int {
	return 1
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
	var ol oligo.Oligo

	if len(data) != 1 {
		return nil, fmt.Errorf("L1: invalid data size %d:%d", len(data), 1)
	}

	if address > c.MaxAddr() {
		return nil, fmt.Errorf("address too big: %d: %d", address, c.MaxAddr())
	}

	val := address
	if ef {
		val += c.MaxAddr()
	}

	val <<= 8
	val |= uint64(data[0])

	ol, err = c.c0.Encode(val)
	if err != nil {
		ret = nil
		return
	}

//	fmt.Printf(">>> address %d ef %v data %d: %d: %v\n", address, ef, data[0], val, ol)
	// append the prefix
	ret = c.prefix.Clone()
	ret.Append(ol)

	// append the suffix
	// FIXME: we don't apply the criteria when appending p3,
	// so theoretically we can have homopolymers etc.
	ret.Append(c.suffix)
	return
}

// Decodes an oligo into the metadata and data it contains
// If the recover parameter is true, try harder to correct the metadata
// Returns a byte array for each data block that was recovered
// (i.e. the parity for the block was correct)
func (c *Codec) Decode(ol oligo.Oligo) (address uint64, ef bool, data []byte, errdist int, err error) {
	var val uint64

	col := c.cutPrimers(ol)
	if col == nil {
		err = fmt.Errorf("primers not found: %v\n", ol)
		return
	}

	val, errdist, err = c.c0.Decode(col)
	if err != nil {
		return
	}

	data = []byte { byte(val)}
	address = val >> 8
	if address > c.MaxAddr() {
		address -= c.MaxAddr()
		ef = true
	}

//	fmt.Printf("<<< %v %d address %d ef %v data %d\n", col, val, address, ef, data[0])
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
