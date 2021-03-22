package main

import (
_	"fmt"
	"runtime"
	"acoma/oligo"
	"acoma/utils"
)

type Cluster struct {
	Diameter	int
	Oligos		[]oligo.Oligo
}

type set struct {
	m	map[int]bool
}

func FindClusters(p *utils.Pool, maxdist int) (clusters []*Cluster) {
	p.InitSearch()

	rchan := make(chan *Cluster)
	nproc := p.Parallel(128, func(ols []oligo.Oligo) {
			for _, ol := range ols {
				matches := p.Search(ol, maxdist)
				ret := make([]oligo.Oligo, 0, len(matches))
				for _, m := range matches {
					ret = append(ret, m.Seq)
				}

				diameter := 0
				for i, o1 := range ret {
					for j := i + 1; j < len(ret); j++ {
						o2 := ret[j]
						d := oligo.Distance(o1, o2)
						if d > diameter {
							diameter = d
						}
					}
				}

				rchan <- &Cluster{diameter, ret}
			}

			rchan <- nil
		})

	var cs []*Cluster
	smap := make(map[oligo.Oligo][]int)
	for n := 0; n < nproc; {
		c := <-rchan
		if c == nil {
			// a gorotine finished
			n++
			continue
		}

		if len(c.Oligos) > 0 {
			// mark all sequences as processed
			cid := len(cs)
			for _, s := range c.Oligos {
				smap[s] = append(smap[s], cid)
			}

			cs = append(cs, c)
		}
	}

	mmap := make(map[int]*set)
	for _, cids := range smap {
		// find the union of all cids so far
		s := new(set)
		s.m = make(map[int]bool)
		for _, cid := range cids {
			s.m[cid] = true
			if cm, found := mmap[cid]; found {
				for c, _ := range cm.m {
					s.m[c] = true
					
				}
			}
		}

		for cid, _ := range s.m {
			mmap[cid] = s
		}
	}

	// trim the map
	rmmap := make(map[*set]bool)
	for _, s := range mmap {
		rmmap[s] = true
	}
		
	var cseqs [][]oligo.Oligo
	for m, _ := range rmmap {
		sm := make(map[oligo.Oligo]bool)
		for cid, _ := range m.m {
			for _, s := range cs[cid].Oligos {
				sm[s] = true
			}
		}

		cseq := make([]oligo.Oligo, 0, len(sm))
		for s, _ := range sm {
			cseq = append(cseq, s)
		}

		cseqs = append(cseqs, cseq)
	}

	pch := make(chan []oligo.Oligo)
	rch := make(chan *Cluster)
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for {
				seqs := <- pch
				if seqs == nil {
					return
				}

				diameter := 0
				for i, s := range seqs {
					for j := i + 1; j < len(seqs); j++ {
						d := oligo.Distance(s, seqs[j])
						if d > diameter {
							diameter = d
						}
					}
				}

				rch <- &Cluster{diameter, seqs}
			}
		}()
	}

	var ncs []*Cluster
	// send all the sequences and receive any clusters back that are available
	for _, seqs := range cseqs {
		for seqs != nil {
			select {
			case pch <- seqs:
				seqs = nil
			case c := <- rch:
				ncs = append(ncs, c)
			}
		}
	}

	// receive the rest of the clusters
	for len(ncs) < len(cseqs) {
		c := <- rch
		ncs = append(ncs, c)
	}

	// close the goroutines
	for i := 0; i < runtime.NumCPU(); i++ {
		pch <- nil
	}

	clusters = ncs
	return
}
