package l0

import (
	"fmt"
_	"math"
	"adscodex/criteria"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/errmdl"
)

const (
	VariantNum = 8
)

// Lookup tables for all prefixes
type LookupTable struct {
	oligoLen	int		// oligo length
	maxVal	int		// maximum value to be used
	pfxLen	int		// prefix length
	vrntNum	int		// number of variants in the decoding tables
	crit	criteria.Criteria
	emdl	errmdl.ErrMdl
	minerr	float64
	etbls	[]*EncTable
	dtbls	[]*DecTable
}

type EncTable struct {
	oligos	[]short.Oligo	// Oligos per each value. The array has maxVal length.
}

type DecVariant struct {
	Val	uint16		// value
	Ol	short.Oligo	// oligo
	Prob	float32		// not sure we need it
}
	
type DecTable struct {
	entries	[][VariantNum]DecVariant	// The array length is 2*oligoLen
}

func newLookupTable(oligoLen, maxVal, pfxLen int, minerr float64, crit criteria.Criteria, emdl errmdl.ErrMdl) *LookupTable {
	tbl := new(LookupTable)
	tbl.oligoLen = oligoLen
	tbl.maxVal = maxVal
	tbl.pfxLen = pfxLen
	tbl.minerr = minerr
	tbl.vrntNum = VariantNum
	tbl.crit = crit
	tbl.emdl = emdl
	tbl.etbls = make([]*EncTable, 1<<(2*pfxLen))
	tbl.dtbls = make([]*DecTable, 1<<(2*pfxLen))

	fmt.Printf("lookup tables: %d olen %d maxval %d\n", 1<<(2*pfxLen), tbl.oligoLen, tbl.maxVal)
	return tbl
}

// encodes a value val to be appended after a prefix
func (lt *LookupTable) getEncode(prefix *short.Oligo, val uint16) oligo.Oligo {
	return &lt.etbls[prefix.Uint64()].oligos[val]
}

// find the most likely decodes from a oligo
func (lt *LookupTable) getDecodes(prefix, ol *short.Oligo) []DecVariant {
	dv := lt.dtbls[prefix.Uint64()].entries[ol.Uint64()]
	for n := 0; n < len(dv); n++ {
		if dv[n].Ol.Len() == 0 {
			return dv[0:n]
		}
	}

	return dv[:]
}

func (t *EncTable) String() (ret string) {
	for i := 0; i < len(t.oligos); i++ {
		ret += fmt.Sprintf("\t%d\t%v\n", i, &t.oligos[i])
	}

	return
}

func (t *DecTable) String(olen int) (ret string) {
	for i := 0; i < len(t.entries); i++ {
		ol := short.Val(olen, uint64(i))
		ret += fmt.Sprintf("\t%v\n", ol)
		for j := 0; j < len(t.entries[i]); j++ {
			v := &t.entries[i][j]
			ret += fmt.Sprintf("\t\t%d\t%v\t%v\n", v.Val, &v.Ol, v.Prob)
		}
	}

	return
}
