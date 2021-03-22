package utils

import (
	"math"
	"runtime"
	"sort"
	"adscodex/oligo"
)

type Pool struct {
	oligos	[]*Oligo

	trie	*Trie
}

func NewPool(ols []oligo.Oligo, unique bool) *Pool {
	var umap map[string] *Oligo

	p := new(Pool)
	if unique {
		umap = make(map[string] *Oligo)
	}

	for _, o := range ols {
		ol, _ := Copy(o)
		if unique {
			s := ol.String()
			if oo, found := umap[s]; found {
				oo.Inc(ol.count, ol.qubund)
				continue
			} else {
				umap[s] = ol
			}
		}

		p.oligos = append(p.oligos, ol)
	}

	return p
}

func ReadPool(fnames []string, unique bool, parse func(string, func(id, sequence string, quality []byte, reverse bool) error) error) (p *Pool, err error) {
	var umap map[string] *Oligo

	p = new(Pool)
	if unique {
		umap = make(map[string] *Oligo)
	}

	for _, fn := range fnames {
		err = parse(fn, func(id, sequence string, quality []byte, reverse bool) error {
			var qubu []float64

			if quality != nil {
				qubu = make([]float64, len(quality))
				for i, q := range quality {
					qubu[i] = 1 - PhredQuality(q)
				}
			}
			ol, ok := FromString(sequence, qubu)
			if !ok {
				// skip
				return nil
			}

			if reverse {
				ol.Reverse()
				ol.Invert()
			}

			if unique {
				s := ol.String()
				if o, found := umap[s]; found {
					o.Inc(ol.count, ol.qubund)
					return nil
				} else {
					umap[s] = ol
				}
			}

			p.oligos = append(p.oligos, ol)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return
}

func (p *Pool) Remove(oligos []*Oligo) {
	omap := make(map[*Oligo] bool)
	for _, o := range oligos {
		omap[o] = true
	}

	var n int
	for i, o := range p.oligos {
		if omap[o] {
			p.oligos[i] = nil
			n++
		}
	}

	noligos := make([]*Oligo, 0, len(p.oligos) - n)
	for _, o := range p.oligos {
		if o != nil {
			noligos = append(noligos, o)
		}
	}

	p.oligos = noligos
	p.trie = nil
}

func (p *Pool) Parallel(procnum int, f func(ols []*Oligo)) (pn int) {
	ncpu := runtime.NumCPU()
	if procnum > ncpu {
		procnum = ncpu
	}

	if len(p.oligos)*10 < procnum {
		procnum = 1
	}

	sperproc := 1 + len(p.oligos)/procnum
	for i := 0; i < procnum; i++ {
		start := i*sperproc
		end := (i+1) * sperproc
		if start >= len(p.oligos) {
			break
		}

		if end > len(p.oligos) {
			end = len(p.oligos)
		}

		go f(p.oligos[start:end])
		pn++
	}

	return
}

// Trims everything outside of the prefix and the suffix. If the oligo doesn't
// have neither suffix nor prefix, it is discarded.
// dist specifies the Levenshtein distance allowed in the prefix and suffix
// and still match them.
// If keep is true, the *ixes are left, otherwise they are also removed
func (p *Pool) Trim(prefix, suffix oligo.Oligo, dist int, keep bool) {
	var oligos []*Oligo

	umap := make(map[string] *Oligo)
	for _, ol := range p.oligos {
		var ppos, spos, plen, slen int

		if prefix != nil {
			ppos, plen = oligo.Find(ol, prefix, dist)
			if ppos < 0 {
				// doesn't match the prefix, skip
				continue
			}
		}

		if suffix != nil {
			spos, slen = oligo.Find(ol, suffix, dist)
			if spos < 0 {
				// doesn't match suffix, skip
				continue
			}
		} else {
			spos = ol.Len()
		}

		if !keep {
			ppos += plen
		} else {
			spos += slen
		}

		o := ol.Slice(ppos, spos).(*Oligo)
		s := o.String()
		if oo, found := umap[s]; found {
			oo.Inc(o.count, o.qubund)
		} else {
			umap[s] = o
			oligos = append(oligos, ol)
		}
	}

	p.oligos = oligos
}

func (p *Pool) Oligos() []*Oligo {
	return p.oligos
}

func (p *Pool) Size() int {
	return len(p.oligos)
}

func (p *Pool) InitSearch() (err error) {
	if p.trie != nil {
		return nil
	}

	p.trie, err = NewTrie(p.oligos)
	return
}

func (p *Pool) Search(ol oligo.Oligo, dist int) (match []DistSeq) {
	if p.trie == nil {
		panic("InitSearch has to be called before Search can be used")
	}

	return p.trie.Search(ol, dist)
}

func (p *Pool) SearchMin(ol oligo.Oligo) (match *DistSeq) {
	if p.trie == nil {
		panic("InitSearch has to be called before Search can be used")
	}

	return p.trie.SearchMin(ol)
}

func (p *Pool) Sort() {
	sort.Slice(p.oligos, func(i, j int) bool {
		return p.oligos[i].Qubundance() > p.oligos[j].Qubundance()
	})
}

func TrimOligo(ol, prefix, suffix oligo.Oligo, dist int, keep bool) oligo.Oligo {
	var ppos, spos, plen, slen int

	if prefix != nil {
		ppos, plen = oligo.Find(ol, prefix, dist)
		if ppos < 0 {
			return nil
		}
	}

	if suffix != nil {
		spos, slen = oligo.Find(ol, suffix, dist)
		if spos < 0 {
			return nil
		}
	} else {
		spos = ol.Len()
	}

	if !keep {
		ppos += plen
	} else {
		spos += slen
	}

	return ol.Slice(ppos, spos)
}

func PhredQuality(q byte) float64 {
	return math.Pow(10, -float64(q)/10)
}
