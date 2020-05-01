package l0

import (
	"fmt"
	"acoma/criteria"
	"acoma/oligo"
	"acoma/oligo/short"
)

// Lookup tables for all prefixes
type LookupTable struct {
	oligolen	int		// oligo length
	pfxlen		int		// prefix length
	crit		criteria.Criteria
	pfxtbl		[]*Table	// tables for each of the prefixes
}

// Lookup table for a single prefix
//
// For encoding tables, the table helps converting the numerical value that
// needs to be encoded to a (short) oligo. The way it works is removing the
// last "bits" bits from tne value and using it as an index in the "tbl"
// array. The value from the array is used as a short oligo and the slow
// counting continues until the original value is reached.
//
// For decoding tables, the table helps converting an oligo to the original
// numerical value that was encoded. The way it works is removing the last
// "bits"/2 nts from the (short) sequence and using it as an index in the "tbl"
// array. The value from the array is used as a starting point for the slow
// counting that calculates the original value.

type Table struct {
	bits	int			// how many bits to skip
	prefix	*short.Oligo		// prefix
	tbl	[]uint64		// 
	maxval	uint64			// maximum value (used for debugging)
}

func newTable(prefix *short.Oligo, bits int) *Table {
	tbl := new(Table)
	tbl.prefix = prefix
	tbl.bits = bits

	return tbl
}

// Looks up a value in the encoding table
// Returns the starting oligo for the "slow" counting as well as 
// how many more values need to be counted.
func (lt *LookupTable) encodeLookup(prefix oligo.Oligo, val uint64) (o oligo.Oligo, rest uint64) {
	sp, ok := short.Copy(prefix)
	if !ok {
		return short.New(lt.oligolen), val
	}

	tbl := lt.pfxtbl[sp.Uint64()]
	idx := val >> tbl.bits
	if idx > uint64(len(tbl.tbl)) {
		idx = uint64(len(tbl.tbl) - 1)
	}

	o, _ = short.Val(lt.oligolen, tbl.tbl[idx]), val - (idx<<tbl.bits)
	rest = val - (idx << tbl.bits)
	return
}

// Looks up an oligo in the decoding table
// Returns the starting oligo for the "slow" counting as well
// as the value associated with that oligo
func (lt *LookupTable) decodeLookup(prefix, oo oligo.Oligo) (o oligo.Oligo, val uint64) {
	sp, ok := short.Copy(prefix)
	if !ok {
		return short.New(lt.oligolen), val
	}

	tbl := lt.pfxtbl[sp.Uint64()]

	so, ok := short.Copy(oo)
	if !ok {
		return short.New(lt.oligolen), val
	}

	if tbl == nil {
		fmt.Printf("decodeLookup prefix %v oligo %v\n", prefix, oo)
		panic("tbl is null")
	}

	if so == nil {
		fmt.Printf("decodeLookup prefix %v oligo %v\n", prefix, oo)
		panic("so is null")
	}

	idx := so.Uint64() >> tbl.bits
	if idx > uint64(len(tbl.tbl)) {
		idx = uint64(len(tbl.tbl) - 1)
	}

	o = short.Val(lt.oligolen, idx<<tbl.bits)
	val = tbl.tbl[idx]
	return
}

// Convert the table to a string (for debugging)
func (tbl *Table) String(oligolen int) string {
	s := fmt.Sprintf("Table %p bits %d prefix %v maxval %d\n", tbl, tbl.bits, tbl.prefix, tbl.maxval)
	for i, v := range tbl.tbl {
		s += fmt.Sprintf("\t%d: %v\n", i, short.Val(oligolen, v))
	}

	return s
}

// Convert the lookup table to a string (for debugging)
func (lt *LookupTable) String() string {
	s := fmt.Sprintf("LookupTable %p oligolen %d pfxlen %d pfxtbl %d\n", lt, lt.oligolen, lt.pfxlen, len(lt.pfxtbl))
	for p := 0; p < len(lt.pfxtbl); p++ {
		s += fmt.Sprintf("%v ", short.Val(4, uint64(p)))
		tbl := lt.pfxtbl[p]
		if tbl == nil {
			s += "\n"
			continue
		}
		s += tbl.String(lt.oligolen)
	}

	return s
}
