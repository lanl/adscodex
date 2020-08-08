package utils

import (
_	"fmt"
	"acoma/oligo"
	"acoma/oligo/long"
)

type Oligo struct {
	ol	oligo.Oligo
	count	int		// number of reads with the same content
	qubund	[]float64	// qubundance (sum of the qualities at the position for all reads)
}

func FromString(s string, qubu []float64) (o *Oligo, ok bool) {
//	fmt.Printf("%v %v\n", s, qubu)
	o = new(Oligo)
	o.count = 1
	o.ol, ok = long.FromString(s)
	if !ok {
		o = nil
		return
	}

	if qubu != nil {
		if o.ol.Len() != len(qubu) {
			o = nil
			return
		}

		o.qubund = make([]float64, len(qubu))
		copy(o.qubund, qubu)
	}

	return
}

func Copy(o oligo.Oligo) (ret *Oligo, ok bool) {
	ret = new(Oligo)

	if ol, ok := o.(*Oligo); ok {
		ret.ol, _ = long.Copy(ol.ol)
		ret.count = ol.count
		if ol.qubund != nil {
			ret.qubund = make([]float64, len(ol.qubund))
			copy(ret.qubund, ol.qubund)
		}
	} else {
		ret.ol, _ = long.Copy(o)
		ret.count = 1
	}

	ok = true
	return
}


// oligo.Oligo interface functions
func (o *Oligo) Len() int {
	return o.ol.Len()
}

func (o *Oligo) String() string {
	return o.ol.String()
}

func (o *Oligo) Cmp(other oligo.Oligo) int {
	return o.ol.Cmp(other)
}

func (o *Oligo) Next() bool {
	return o.ol.Next()
}

func (o *Oligo) At(idx int) int {
	return o.ol.At(idx)
}

func (o *Oligo) Set(idx int, nt int) {
	o.ol.Set(idx, nt)
}

func (o *Oligo) Slice(start, end int) oligo.Oligo {
	olen := o.ol.Len()
	if end <= 0 {
		end = olen - end
	}

	if end > olen {
		end = olen
	} else if end < 0 {
		end = 0
	}

	if start < 0 || start > olen || start > end { 
		return &Oligo{ nil, 0, nil }
	}

	ol := new(Oligo)
	ol.ol = o.ol.Slice(start, end)

	if o.qubund != nil {
		ol.qubund = make([]float64, ol.ol.Len())
		copy(ol.qubund, o.qubund[start:end])
	}

	return ol
}

func (o *Oligo) Clone() oligo.Oligo {
	ol := new(Oligo)
	ol.ol = o.ol.Clone()
	if o.qubund != nil {
		ol.qubund = make([]float64, len(o.qubund))
		copy(ol.qubund, o.qubund)
	}

	return ol
}

func (o *Oligo) Append(other oligo.Oligo) bool {
	if !o.ol.Append(other) {
		return false
	}

	if qo, ok := other.(*Oligo); ok {
		if o.qubund != nil && qo.qubund != nil {
			o.qubund = append(o.qubund, qo.qubund...)
		}
	} else {
		// we don't know the quality of the appended oligo, so get rid of it all
		o.qubund = nil
	}

	return true
}

// extra functions

// additional read(s) of the same oligo
func (o *Oligo) Inc(count int, qubu []float64) bool {
	if o.qubund != nil {
		if len(o.qubund) != len(qubu) {
			return false
		}

		for i, q := range qubu {
			o.qubund[i] += q
		}
	}

	o.count += count
	return true
}

func (o *Oligo) Count() int {
	return o.count
}

func (o *Oligo) Qubundance() float64 {
	return o.Quality() * float64(o.count)
}

func (o *Oligo) Quality() (ret float64) {
	ret = 1
	if o.qubund != nil {
		for _, q := range o.qubund {
			ret *= q/float64(o.count)
		}
	}

	return
}

func (o *Oligo) QubundanceAt(idx int) float64 {
	if o.qubund != nil {
		return o.qubund[idx]
	} else {
		return float64(o.count)
	}
}

func (o *Oligo) QualityAt(idx int) float64 {
	return o.QubundanceAt(idx) / float64(o.count)
}

func (o *Oligo) Qubundances() []float64 {
	return o.qubund
}

func  (o *Oligo) Reverse() {
	oligo.Reverse(o)

	// reverse the quality array
	for n, i := len(o.qubund), 0; i < n/2; i++ {
		o.qubund[i], o.qubund[n - i - 1] = o.qubund[n - i - 1], o.qubund[i]
	}
}

func  (o *Oligo) Invert() {
	oligo.Invert(o)
}

func (o *Oligo) Trim(prefix, suffix oligo.Oligo, dist int, keep bool) oligo.Oligo {
	var ppos, spos, plen, slen int

	if prefix != nil {
		ppos, plen = oligo.Find(o, prefix, dist)
		if ppos < 0 {
			return nil
		}
	}

	if suffix != nil {
		spos, slen = oligo.Find(o, suffix, dist)
		if spos < 0 {
			return nil
		}
	} else {
		spos = o.Len()
	}

	if !keep {
		ppos += plen
	} else {
		spos += slen
	}

	return o.Slice(ppos, spos)
}
