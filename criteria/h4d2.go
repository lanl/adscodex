package criteria

import (
	"adscodex/oligo"
)

type h4d2 int
var H4D2 h4d2

func (h4d2) Id() uint64 {
	return 'H'<<24 | '4'<<16 | '/' << 8 | '2'
}

func (h4d2) FeatureLength() int {
	return 4
}

func (h4d2) String() string {
	return "h4/2"
}

func (h4d2) Check(o oligo.Oligo) bool {
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
			} else if nt == oligo.G {
				if i == 1 || n > 2 {
					return false
				}
			} else if i == 2 && n > 2 {
				return false
			}
		} else {
			snt = nt
			n = 1
		}
	}

	if (snt == oligo.G && n > 1) || n > 2 {
		return false
	}

	return true
}
