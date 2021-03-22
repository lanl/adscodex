package l0

import (
	"acoma/criteria"
	"acoma/oligo/short"
)

// Build an encoding table for the specified prefix, oligo length and criteria
// Right now this all works for short oligos only
func BuildEncodingTable(prefix *short.Oligo, olen, bits int, c criteria.Criteria) (tbl *Table) {
	var n uint64

	tbl = newTable(prefix, bits)
	o := prefix.Clone()
	o.Append(short.New(olen))

	olast := prefix.Clone()
	olast.Next()
	olast.Append(short.New(olen))

	tbl.tbl = make([]uint64, 0, 1<<(2*olen - bits))
	for o.Cmp(olast) < 0 {
		if c.Check(o) {
			idx := int(n >> bits)
			if idx >= len(tbl.tbl) {

				s, ok := short.Copy(o.Slice(4, 0))
				if !ok {
					panic("value too big")
				}

				tbl.tbl = append(tbl.tbl, s.Uint64())
			}
			n++
		}

		if !o.Next() {
			break
		}
	}

	tbl.maxval = n
	return
}

// Build a decoding table for the specified prefix, oligo length and criteria
func BuildDecodingTable(prefix *short.Oligo, olen, nts int, c criteria.Criteria) (tbl *Table) {
	var n uint64

	bits := 2 * nts
	tbl = newTable(prefix, bits)
	o := prefix.Clone()
	o.Append(short.New(olen))

	olast := prefix.Clone()
	olast.Next()
	olast.Append(short.New(olen))

	for o.Cmp(olast) < 0 {
		s, ok := short.Copy(o.Slice(4, 0))
		if !ok {
			panic("value too big")
		}

		idx := int(s.Uint64() >> bits)
		if idx >= len(tbl.tbl) {
			tbl.tbl = append(tbl.tbl, n)
		}

		if c.Check(o) {
			n++
		}

		if !o.Next() {
			break
		}
	}

	tbl.maxval = n
	return
}

// build encoding tables for all different prefixes
func BuildEncodingLookupTable(pfxlen, olen, bits int, c criteria.Criteria) (ltbl *LookupTable) {
	ltbl = new(LookupTable)
	ltbl.oligolen = olen
	ltbl.pfxlen = pfxlen
	ltbl.crit = c

	prefix := (*short.Oligo)(short.New(pfxlen))
	ltbl.pfxtbl = make([]*Table, (1<<(pfxlen*2)))
	done := make(chan uint64)
	for {
		go func(pfx *short.Oligo) {
			idx := pfx.Uint64()
			ltbl.pfxtbl[idx] = BuildEncodingTable(pfx, olen, bits, c)
			done <- idx
		}(prefix.Clone().(*short.Oligo))

		if !prefix.Next() {
			break
		}
	}


	for i := 0; i < len(ltbl.pfxtbl); i++ {
		<-done
	}

	ltbl.maxval = int64(ltbl.MaxVal())
	return ltbl
}

// build decoding tables for all different prefixes
func BuildDecodingLookupTable(pfxlen, olen, bits int, c criteria.Criteria) (ltbl *LookupTable) {
	ltbl = new(LookupTable)
	ltbl.oligolen = olen
	ltbl.pfxlen = pfxlen
	ltbl.crit = c

	prefix := (*short.Oligo)(short.New(pfxlen))
	ltbl.pfxtbl = make([]*Table, (1<<(pfxlen*2)))
	done := make(chan uint64)
	for {
		go func(pfx *short.Oligo) {
			idx := pfx.Uint64()
			ltbl.pfxtbl[idx] = BuildDecodingTable(pfx, olen, bits, c)
			done <- idx
		}(prefix.Clone().(*short.Oligo))

		if !prefix.Next() {
			break
		}
	}


	for i := 0; i < len(ltbl.pfxtbl); i++ {
		<-done
	}

	ltbl.maxval = int64(ltbl.MaxVal())
	return
}
