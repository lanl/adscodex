package l0

/*
import (
	"adscodex/criteria"
)

func MaxVal(oligoLen int, c criteria.Criteria) int64 {
	tbl := getEncodeTable(oligoLen, c)
	if tbl == nil {
		return -1
	}

	return tbl.maxval
}

func MaxBits(oligoLen int, c criteria.Criteria) int {
	tbl := getEncodeTable(oligoLen, c)
	if tbl == nil {
		return -1
	}

	// TODO: we can store the value in the lookup table if we need it faster
	v := MaxVal(oligoLen, c)
	if v <= 0 {
		return -1
	}

	var n int
	for n = 0; v != 0; n++ {
		v >>= 1
	}

	return n
}

// what oligoLen can store the specified number of bits
func Bits2OligoLen(bitnum int, c criteria.Criteria) int {
	oligoLen := 2*bitnum
	for MaxBits(oligoLen, c) < bitnum {
		oligoLen++
	}

	return oligoLen
}
*/
