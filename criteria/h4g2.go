package criteria

import (
	"acoma/oligo"
)

type h4g2 int
var H4G2 h4g2

func (h4g2) Check(o oligo.Oligo) bool {
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
			} else if nt == oligo.G && n > 2 {
				return false
			}
		} else {
			snt = nt
			n = 1
		}
	}

	return true
}
