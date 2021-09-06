package l0

import (
	"fmt"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/criteria"
	"adscodex/errmdl"
)

type Codec struct {
	lt	*LookupTable
}

// reads a codec from a file
func Load(fname string) (c *Codec, err error) {
	c = new(Codec)
	c.lt, err = readLookupTable(fname)
	if err != nil {
		c = nil
	}

	return
}

func New(oligoLen, maxVal, pfxLen int, minerr float64, crit criteria.Criteria, emdl errmdl.ErrMdl) (c *Codec, err error) {
	c = new(Codec)
	c.lt = BuildLookupTable(oligoLen, maxVal, pfxLen, minerr, crit, emdl)

	return
}

// Encodes the value into an oligo.
// Returns error if the value can't be encoded
func (c *Codec) Encode(prefix oligo.Oligo, val uint64) (o oligo.Oligo, err error) {
	if uint64(c.lt.maxVal) <= val {
		return nil, fmt.Errorf("value (%d) is greater than supported by the codec (%d)", val, c.lt.maxVal)
	}

	pfx, ok := short.Copy(prefix)
	if !ok {
		return nil, fmt.Errorf("prefix too long")
	}

	return &c.lt.etbls[pfx.Uint64()].oligos[val], nil
}

// Decodes the specified oligo into a value
// Returns all possible variants for decoding
func (c *Codec) Decode(prefix, o oligo.Oligo) (vals []DecVariant, err error) {
	if o.Len() != c.lt.oligoLen {
		return nil, fmt.Errorf("invalid oligo length: got %d expected %d", o.Len(), c.lt.oligoLen)
	}

	pfx, ok := short.Copy(prefix)
	if !ok {
		return nil, fmt.Errorf("prefix too long")
	}

	ol, ok := short.Copy(o)
	if !ok {
		return nil, fmt.Errorf("oligo too long")
	}

	olnum := ol.Uint64()
//	fmt.Printf("+++ olnum %d pfx %d dtbls %d\n", olnum, pfx.Uint64(), len(c.lt.dtbls))
	dtbl := c.lt.dtbls[pfx.Uint64()]
//	fmt.Printf("---olnum %d %p\n", olnum, dtbl) // len(dtbl.entries))
	vs := &dtbl.entries[olnum]
	return vs[:], nil
}

func (c *Codec) OligoLen() int {
	return c.lt.oligoLen
}

func (c *Codec) MaxVal() int {
	return c.lt.maxVal
}

func (c *Codec) PrefixLen() int {
	return c.lt.pfxLen
}

func (c *Codec) EncodeTable(pfx uint64) *EncTable {
	return c.lt.etbls[pfx]
}

func (c *Codec) DecodeTable(pfx uint64) *DecTable {
	return c.lt.dtbls[pfx]
}

