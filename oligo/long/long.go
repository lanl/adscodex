// Package oligo/long implements the Oligo interface for sequences of
// any length.
package long

import (
	"acoma/oligo"
)

type Oligo struct {
	// Oligo length
	len	int

	// Sequence of nts
	// Each nt uses one byte with nt at position 0 stored 
	// in seq[0], etc.
	seq	[]byte
}

// Creates a new long oligo object with the specified length and
// value of "AAA...AA"
func New(olen int) *Oligo {
	return &Oligo {olen, make([]byte, olen) }
}

// Creates a new short oligo object with the specified length and oligo value
// Returns an Object and true if the conversion was successful
func FromString(s string) (*Oligo, bool) {
	var seq []byte

	for _, c := range s {
		nt := oligo.String2Nt(string(c))
		if nt < 0 {
			return nil, false
		}

		seq = append(seq, byte(nt))
	}

	return &Oligo{len(seq), seq}, true
}

// for when we know that there can't be error
func FromString1(s string) *Oligo {
	o, _ := FromString(s)
	return o
}

// Copies oligo of any type (implementing the Oligo interface) to a short
// Oligo (if possible). 
// Returns a new Oligo object and true (because the conversion is alwys possible)
func Copy(o oligo.Oligo) (*Oligo, bool) {
	ol := new(Oligo)
	ol.len = o.Len()
	ol.seq = make([]byte, ol.len)
	for i := 0; i < ol.len; i++ {
		ol.seq[i] = byte(o.At(i))
	}

	return ol, true
}

// Implementation of the Oligo interface...
func (o *Oligo) Len() int {
	return o.len
}

func (o *Oligo) String() (ret string) {
	for i := 0; i < o.len; i++ {
		ret = ret + oligo.Nt2String(o.At(i))
	}

	return ret
}

func (o *Oligo) Cmp(other oligo.Oligo) int {
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
	var i int

	for i = o.len - 1; i >= 0; i-- {
		o.seq[i]++
		if o.seq[i] > 3 {
			o.seq[i] = 0
		} else {
			break
		}
	}

	return i >= 0
}

func (o *Oligo) At(idx int) int {
	if idx < 0 || idx > o.len {
		return -1
	}

	return int(o.seq[idx])
}

func (o *Oligo) Slice(start, end int) oligo.Oligo {
//	fmt.Printf("Slice: %v start %d end %d: %v\n", o, start, end, o.seq)
	if end <= 0 {
		end = o.len - end
	}

	if end > o.len {
		end = o.len
	} else if end < 0 {
		end = 0
	}

	if start < 0 || start > o.len || start > end { 
		return &Oligo{ 0, nil }
	}

	no := new(Oligo)
	no.len = end - start
	no.seq = make([]byte, no.len)
	copy(no.seq, o.seq[start:end])

	return no
}

func (o *Oligo) Clone() oligo.Oligo {
	no := new(Oligo)
	no.len = o.len
	no.seq = make([]byte, no.len)
	copy(no.seq, o.seq)

	return no
}

func (o *Oligo) Append(other oligo.Oligo) bool {
/*
	othlen := other.Len()
	seq := make([]byte, o.len + othlen)
	copy(seq[othlen:], o.seq)
	for i := 0; i < other.Len(); i++ {
		seq[i] = byte(other.At(i))
	}

	o.len += other.Len()
	o.seq = seq
*/
	o.len += other.Len()
	for i := 0; i < other.Len(); i++ {
		o.seq = append(o.seq, byte(other.At(i)))
	}

	return true
}

