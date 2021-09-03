// Package oligo/short implements the Oligo interface for short sequences.
// It stores the oligo into a 64-bit value, and therefore can store oligos
// up to 32 nts long.
package short

import (
	"adscodex/oligo"
)

type Oligo struct {
	// Oligo length
	len	int

	// Sequence of nts
	// Each nt uses 2 bits of the value, with nts stored in
	// reverse order, with nt at position len-1 at bits [1:0] and
	// nt at position 0 at bits [1<<(2*(len-1)):1<<(2*(len-1)-1)]
	seq	uint64
}

// Creates a new short oligo object with the specified length and
// value of "AAA...AA"
func New(olen int) *Oligo {
	return &Oligo {olen, 0}
}

// Creates a new short oligo object with the specified length and oligo value
func Val(olen int, val uint64) *Oligo {
	return &Oligo {olen, val}
}

// Converts a string representation of an oligo to an Oligo object
// Returns an Object and true if the conversion was successful
func FromString(s string) (*Oligo, bool) {
	var v uint64

	if len(s) >= 32 {
		return nil, false
	}

	for _, c := range s {
		nt := oligo.String2Nt(string(c))
		if nt < 0 {
			return nil, false
		}

		v = (v<<2) | uint64(nt)
	}

	return Val(len(s), v), true
}

// if we don't care about fitting
func FromString1(s string) (o *Oligo) {
	o, _ = FromString(s)
	return
}

// Copies oligo of any type (implementing the Oligo interface) to a short
// Oligo (if possible). 
// Returns a new Oligo object and true, if the conversion was possible.
func Copy(o oligo.Oligo) (*Oligo, bool) {
	var v uint64

	if o.Len() >= 32 {
		return nil, false
	}

	for i := 0; i < o.Len(); i++ {
		v = (v<<2) | uint64(o.At(i))
	}

	return Val(o.Len(), v), true
}


// Implementation of the Oligo interface...
func (o *Oligo) Len() int {
	return o.len
}

func (o *Oligo) String() (ret string) {
	for i, s := 0, o.seq; i < o.len; i++ {
		ret, s = oligo.Nt2String(int(s & 0x3)) + ret, s>>2
	}

	return ret
}

func (o *Oligo) Cmp(other oligo.Oligo) int {
	if o1, ok := other.(*Oligo); ok {
		if o.len < o1.len {
			return -1
		} else if o.len > o1.len {
			return 1
		}

		if o.seq < o1.seq {
			return -1
		} else if o.seq > o1.seq {
			return 1
		}

		return 0
	}

	olen := other.Len()
	if o.len < olen {
		return -1
	} else if o.len > olen {
		return 1
	}

	for i := 0; i < o.len; i++ {
		n := o.At(i) - other.At(i)
		if n < 0 {
			return -1
		} else if n > 0 {
			return 1
		}
	}

	return 0
}

func (o *Oligo) Next() bool {
	max := (uint64(1)<<(2*o.len)) - 1

	if o.len == 32 || o.seq == max {
		return false
	}

	o.seq++

	return true
}

func (o *Oligo) At(idx int) int {
//	if idx < 0 || idx > o.len {
//		return -1
//	}

	return int((o.seq>>(2*(o.len - idx - 1))) & 0x3)
}

func (o *Oligo) Set(idx int, nt int) {
	pos := 2*(o.len - idx - 1)
	o.seq &= ^(0x3 << pos)
	o.seq |= uint64((nt & 0x3)) << pos
}

func (o *Oligo) Slice(start, end int) oligo.Oligo {
	if end <= 0 {
		end = o.len - end
	}

	if end > o.len {
		end = o.len
	} else if end < 0 {
		end = 0
	}

	if start < 0 || start > o.len || start > end { 
		return nil
	}

	olen := end - start
	omask := uint64((1<<(2*olen)) - 1)
	return &Oligo{ olen, (o.seq >> (2*(o.len - end))) & omask }
}

func (o *Oligo) Clone() oligo.Oligo {
	return &Oligo{o.len, o.seq}
}

func (o *Oligo) Append(other oligo.Oligo) bool {
	if o.len + other.Len() > 32 {
		return false
	}

	o.len += other.Len()
	for i := 0; i < other.Len(); i++ {
		nt := other.At(i)
		o.seq = (o.seq<<2) | uint64(nt)
	}
	return true
}

func (o *Oligo) Uint64() uint64 {
	return o.seq
}

func (o *Oligo) SetVal(olen int, val uint64) {
	o.len = olen
	o.seq = val
}
