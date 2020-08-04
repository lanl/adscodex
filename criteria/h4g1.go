package criteria

import (
	"acoma/oligo"
)

type h4g1 int
var H4G1 h4g1

func (h4g1) Id() uint64 {
	return 'H'<<24 | '4'<<16 | 'G' << 8 | '1'
}

func (h4g1) FeatureLength() int {
	return 4
}

func (h4g1) String() string {
	return "h4g1"
}

func (h4g1) Check(o oligo.Oligo) bool {
	l := o.Len()
	if l <= 0 {
		return false
	}

	n := 1
	snt := o.At(0)
	for i := 1; i < l; i++ {
		nt := o.At(i)
		if nt == snt {
			n++
			if n > 4 {
				return false
			} else if nt == oligo.G && n > 1 {
				return false
			}
		} else {
			snt = nt
			n = 1
		}
	}

	return true
}
