package l0

import (
	"errors"
	"fmt"
	"os"
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
			return o, fmt.Errorf("value too large: prefix %v len %d val %d current %v:%d end %v", prefix, oo.Len(), val, o, n, oend)
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

func LoadOrGenerateEncodeTable(oligoLen int, c criteria.Criteria) (err error) {
	var fname string

	if getEncodeTable(oligoLen, c) != nil {
		return
	}

	// first look for file that contains the table
	for bits := oligoLen * 2; bits >= 0; bits-- {
		fname = fmt.Sprintf("%s/%s-%02d-%02d.etbl", tblPath, c.String(), oligoLen, bits)
		_, err = os.Stat(fname)
		if err == nil {
			break
		}
		fname = ""
	}

	if fname != "" {
		err = LoadEncodeTable(fname, c)
		return
	}

	// no lookup table on file, generate it
	if oligoLen > 10 {
		// but warn if it will take a long time
		// TODO: should we save it?
		fmt.Fprintf(os.Stderr, "Warning: generation of lookup table for %d nt, it might take a long time...\n", oligoLen)
	}

	err = RegisterEncodeTable(BuildEncodingLookupTable(c.FeatureLength(), oligoLen, oligoLen, c))
	return
}
