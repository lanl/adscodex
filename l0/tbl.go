package l0

import (
	"fmt"
	"os"
	"adscodex/criteria"
	"adscodex/oligo"
	"adscodex/oligo/short"
)

// Lookup tables for all prefixes
type LookupTable struct {
	oligolen	int		// oligo length
	pfxlen		int		// prefix length
	mindist		int		// minimum distance between oligos
	crit		criteria.Criteria
	pfxtbl		[]*Table	// tables for each of the prefixes
	maxval		uint64		// maximum value that can be stored with the criteria at the oligoLen
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
	prefix	*short.Oligo		// prefix
	olen	int			// oligo length
	mindist	int			// minimum distance
	etbl	[]uint64		// encoding table
	dmap	map[uint64]int		// decoding map
	trie	*Trie
	maxval	uint64			// maximum value
}

var lookupTablePath string
var codecTables []*LookupTable;

func newTable(prefix *short.Oligo, olen, mindist int) *Table {
	tbl := new(Table)
	tbl.prefix = prefix
	tbl.olen = olen
	tbl.mindist = mindist
	tbl.dmap = make(map[uint64]int)
	tbl.trie, _ = NewTrie(nil)

	return tbl
}

func getTable(c criteria.Criteria, olen, mindist int) *LookupTable {
	for _, lt := range codecTables {
		if lt.oligolen == olen && lt.crit == c && (mindist < 0 || lt.mindist == mindist) {
			return lt
		}
	}

	return nil
}

func (lt *LookupTable) register() {
	// check if it is already registered, just in case

	for _, lt1 := range codecTables {
		if lt == lt1 {
			return
		}

		if lt1.oligolen == lt.oligolen && lt1.crit == lt.crit && lt1.mindist == lt.mindist {
			return
//			panic("registering the same table?")
		}
	}

	codecTables = append(codecTables, lt)
}

// Looks up a value in the encoding table
func (lt *LookupTable) encodeLookup(prefix oligo.Oligo, val uint64) (o oligo.Oligo) {
	sp, ok := short.Copy(prefix)
	if !ok {
		panic("prefix too long")
	}

	tbl := lt.pfxtbl[sp.Uint64()]
	if uint64(val) > lt.maxval {
		panic("val is too big")
	}

	return short.Val(lt.oligolen, tbl.etbl[val])
}

// Looks up value the decoding table
func (lt *LookupTable) decodeLookup(prefix, oo oligo.Oligo) (val int) {
	sp, ok := short.Copy(prefix)
	if !ok {
		panic("prefix too long")
	}

	tbl := lt.pfxtbl[sp.Uint64()]
	so, ok := short.Copy(oo)
	if !ok {
		panic("oligo is too long")
	}

	val, ok = tbl.dmap[so.Uint64()]
	if !ok {
		panic("oligo not found in the decoding map")
	}

	return
}

func (lt *LookupTable) MaxVal() uint64 {
	return lt.maxval
}

// Convert the table to a string (for debugging)
func (tbl *Table) String(oligolen int) string {
	s := fmt.Sprintf("Table %p prefix %v maxval %d\n", tbl, tbl.prefix, tbl.maxval)
	for i, v := range tbl.etbl {
		s += fmt.Sprintf("\t%d: %v\n", i, short.Val(oligolen, v))
	}

	return s
}

// Convert the lookup table to a string (for debugging)
func (lt *LookupTable) String() string {
	s := fmt.Sprintf("LookupTable %p oligolen %d pfxlen %d mindist %d maxval %d pfxtbl %d\n", lt, lt.oligolen, lt.pfxlen, lt.mindist, lt.maxval, len(lt.pfxtbl))
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

func LookupTableFilename(c criteria.Criteria, olen, mindist int) string {
	return fmt.Sprintf("%s/%s_o%d_m%d.tbl", lookupTablePath, c.String(), olen, mindist)
}

func LoadOrGenerateTable(c criteria.Criteria, olen, mindist int, shuffle bool, maxval int) (lt *LookupTable, err error) {
	lt = getTable(c, olen, mindist)
	if lt != nil {
		return
	}

	lt, err = LoadLookupTable(LookupTableFilename(c, olen, mindist))
	if err == nil {
//		fmt.Printf("lt %p\n", lt)
		return
	}

	fmt.Printf("Warning: generating lookup table, this may take awhile...\n")
	lt = BuildLookupTable(c, olen, mindist, shuffle, maxval)
	return
}

func SetLookupTablePath(p string) {
	lookupTablePath = p
}

func init() {
	if s := os.Getenv("ADSTBLPATH"); s != "" {
		lookupTablePath = s
	} else {
		lookupTablePath = "../tbl"
	}
}
