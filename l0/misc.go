package l0

import (
	"adscodex/criteria"
)

func MaxVal(oligoLen, mindist int, c criteria.Criteria) uint64 {
	tbl := getTable(c, oligoLen, mindist)
	if tbl == nil {
		return 0
	}

	return tbl.maxval
}

func MaxBits(oligoLen, mindist int, c criteria.Criteria) int {
	v := MaxVal(oligoLen, mindist, c)
	if v <= 0 {
		return -1
	}

	var n int
	for n = 0; v != 0; n++ {
		v >>= 1
	}

	return n
}
