package main

import (
	"flag"
	"fmt"
	"os"
_	"runtime"
	"sort"
_	"adscodex/oligo"
_	"adscodex/oligo/long"
	"adscodex/l1"
)

const Maxaddr = 1000000000

var maxdist = flag.Int("maxdist", 0, "filter out entries with large distance (0 - no filter)")

func main() {
	var ents, ret []*l1.Entry
	var err error

	flag.Parse()

	ents, err = l1.ReadEntries(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	emap := make(map[uint64][]*l1.Entry)
	for _, ent := range ents {
		if *maxdist > 0 && ent.Dist > *maxdist {
			continue
		}

		addr := ent.Addr
		if addr >= Maxaddr {
			panic("addr >= Maxaddr")
		}

		if ent.EcFlag {
			addr += Maxaddr
		}

		el := emap[addr]
		el = append(el, ent)
		emap[ent.Addr] = el
	}

	for _, els := range emap {
		// find the consensus values for an address

		me := make(map[string]*l1.Entry)
		mc := make(map[string]int)
		maxcnt := 0
		for _, e := range els {
			v := ""
			for _, b := range e.Data {
				v += fmt.Sprintf("%2x", b)
			}

			if ee := me[v]; ee == nil || ee.Dist > e.Dist {
				me[v] = e
			}

			c := mc[v] + e.Count
			mc[v] = c
			if c > maxcnt {
				maxcnt = c
			}
		}

		for v, c := range mc {
			if c == maxcnt {
				e := me[v]
				e.Count = c
				ret = append(ret, e)
			}
		}
	}

	sort.Slice(ret, func (i, j int) bool {
/*
		if ret[i].Dist < ret[j].Dist {
			return true
		} else if ret[i].Dist == ret[j].Dist {
			return ret[i].Addr < ret[j].Addr
		}

		return false
*/
		return ret[i].Addr < ret[j].Addr
	})

	err = l1.WriteEntries(flag.Arg(1), ret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	return
}
