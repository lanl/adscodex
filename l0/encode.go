package l0

import (
	"errors"
	"fmt"
	"acoma/oligo"
	"acoma/oligo/short"
	"acoma/criteria"
)

var encodeTables map[criteria.Criteria] map[int]*LookupTable;

// Encodes the value into oligo with the specified length
// Returns error if the value can't be encoded
func Encode(prefix oligo.Oligo, val uint64, oligoLen int, c criteria.Criteria) (o oligo.Oligo, err error) {
	var tbl *LookupTable

	if encodeTables != nil {
		// find tables for the criteria (if any)
		ctbl := encodeTables[c]
		if ctbl != nil {
			// find tables for the oligo len (if any)
			tbl = ctbl[oligoLen]
		}
	}

	if tbl != nil {
		// find closest starting point
		o, val = tbl.encodeLookup(prefix, val)
	} else {
		o = short.New(oligoLen)
	}

	// count the rest
	return encodeSlow(prefix, o, val, c)
}

// Encodes the value into oligo with the specified length
// Returns error if the value can't be encoded
func encodeSlow(prefix, oo oligo.Oligo, val uint64, c criteria.Criteria) (o oligo.Oligo, err error) {
	var n uint64

	o = prefix.Clone()
	o.Append(oo)
	oend := prefix.Clone()
	oend.Next()
	oend.Append(oo)

	for {
		if c.Check(o) {
			if n == val {
				break
			}

			n++
		}

		if !o.Next() || o.Cmp(oend) >= 0 {
			return o, fmt.Errorf("value too large: len %d val %d current %v:%d end %v", oo.Len(), val, o, n, oend)
		}

	}

	return o.Slice(prefix.Len(), o.Len()), nil
}

// Loads an encoding table from a file and registers it to be used by the Encode
// function
func LoadEncodeTable(fname string, crit criteria.Criteria) (err error) {
	var lt *LookupTable

	lt, err = readLookupTable(fname, crit)
	if err != nil {
		return
	}
	lt.crit = crit

	return RegisterEncodeTable(lt)
}

// Registers encoding table to be used the Encode function
func RegisterEncodeTable(lt *LookupTable) error {
	if encodeTables == nil {
		encodeTables = make(map[criteria.Criteria] map[int]*LookupTable)
	}

	if encodeTables[lt.crit] == nil {
		encodeTables[lt.crit] = make(map[int]*LookupTable)
	}

	if encodeTables[lt.crit][lt.oligolen] != nil {
		return errors.New("table already loaded")
	}

	encodeTables[lt.crit][lt.oligolen] = lt

	return nil
}
