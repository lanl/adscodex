package criteria

import (
	"adscodex/oligo"
)

type h4 int
var H4 h4

func (h4) Id() uint64 {
	return 'H'<<24 | '0'<<16 | '0' << 8 | '4'
}

func (h4) FeatureLength() int {
	return 4
}

func (h4) String() string {
	return "h4"
}

func (h4) Check(o oligo.Oligo) bool {
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
			}
		} else {
			snt = nt
			n = 1
		}
	}

	return true
}
