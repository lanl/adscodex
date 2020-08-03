package l0

import (
	"errors"
	"fmt"
	"acoma/oligo"
	"acoma/oligo/short"
	"acoma/criteria"
)

var decodeTables map[criteria.Criteria] map[int]*LookupTable;

// Decodes the specified oligo into a value
// Returns error if the value can't be encoded
func Decode(prefix, o oligo.Oligo, c criteria.Criteria) (val uint64, err error) {
	var so oligo.Oligo

	if !c.Check(prefix) {
		err = fmt.Errorf("invalid prefix: %v\n", prefix)
		return
	}

	if !c.Check(o) {
		err = fmt.Errorf("invalid oligo: %v\n", o)
		return
	}

	tbl := getDecodeTable(o.Len(), c)
	if tbl != nil {
		// find closest starting point
		so, val = tbl.decodeLookup(prefix, o)
	} else {
		so = short.New(o.Len())
	}

	// finish counting
	return decodeSlow(prefix, o, so, val, c)
}

// Decodes the specified oligo oo into a value.
// (startol, startval) are the starting points for the oligo counting.
// Returns error if the value can't be encoded
func decodeSlow(prefix, oo, startol oligo.Oligo, startval uint64, c criteria.Criteria) (v uint64, err error) {
	o := prefix.Clone()
	o.Append(startol)

	po := prefix.Clone()
	po.Append(oo)

	oend := prefix.Clone()
	oend.Next()
	oend.Append(short.New(oo.Len()))

	val := startval
	for po.Cmp(o) != 0 {
		if c.Check(o) {
			val++
		}

		if !o.Next() || o.Cmp(oend) >= 0 {
			return 0, errors.New("value too large")
		}
	}

	return val, nil
}

// Load a decoding table from a file and register it for use by the Decode function
func LoadDecodeTable(fname string, crit criteria.Criteria) (err error) {
	var lt *LookupTable

	lt, err = readLookupTable(fname, crit)
	if err != nil {
		return
	}

	lt.crit = crit
	return RegisterDecodeTable(lt)
}

// Registers a decoding table for use by the Decode function
func RegisterDecodeTable(lt *LookupTable) error {
	if decodeTables == nil {
		decodeTables = make(map[criteria.Criteria] map[int]*LookupTable)
	}

	if decodeTables[lt.crit] == nil {
		decodeTables[lt.crit] = make(map[int]*LookupTable)
	}

	if decodeTables[lt.crit][lt.oligolen] != nil {
		return errors.New("table already loaded")
	}

	decodeTables[lt.crit][lt.oligolen] = lt

	return nil
}
