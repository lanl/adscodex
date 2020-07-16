package utils

import (
	"runtime"
	"acoma/oligo"
	"acoma/oligo/long"
)

type Pool struct {
	oligos	[]oligo.Oligo
	olcnt	map[oligo.Oligo] int		// count per each unique oligo

	trie	*Trie
}

func NewPool(ols []oligo.Oligo, unique bool) *Pool {
	var umap map[string] oligo.Oligo

	p := new(Pool)
	if unique {
		p.olcnt =  make(map[oligo.Oligo] int)
		umap = make(map[string] oligo.Oligo)
	}

	for _, ol := range ols {
		if unique {
			s := ol.String()
			if o, found := umap[s]; found {
				p.olcnt[o]++
				continue
			} else {
				umap[s] = ol
				p.olcnt[ol] = 1
			}
		}

		p.oligos = append(p.oligos, ol)
	}

	return p
}

func ReadPool(fnames []string, unique bool, parse func(string, func(id, sequence string, quality []byte, reverse bool) error) error) (p *Pool, err error) {
	var umap map[string] oligo.Oligo

	p = new(Pool)
	if unique {
		p.olcnt =  make(map[oligo.Oligo] int)
		umap = make(map[string] oligo.Oligo)
	}

	for _, fn := range fnames {
		err = parse(fn, func(id, sequence string, quality []byte, reverse bool) error {
			ol, ok := long.FromString(sequence)
			if !ok {
				// skip
				return nil
			}

			if reverse {
				oligo.Reverse(ol)
				oligo.Invert(ol)
			}

			if unique {
				s := ol.String()
				if o, found := umap[s]; found {
					p.olcnt[o]++
					return nil
				} else {
					umap[s] = ol
					p.olcnt[ol] = 1
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

func (p *Pool) Parallel(procnum int, f func(ols []oligo.Oligo)) (pn int) {
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

// Trims everything outside of the prefix and the suffix.
// dist specifies the Levenshtein distance allowed in the prefix and suffix
// and still match them.
// If keep is true, the *ix are left, otherwise they are also removed
func (p *Pool) Trim(prefix, suffix oligo.Oligo, dist int, keep bool) {
	var oligos []oligo.Oligo
	var olcnt map[oligo.Oligo] int
	var umap map[string] oligo.Oligo

	if p.olcnt != nil {
		olcnt = make(map[oligo.Oligo] int)
		umap = make(map[string] oligo.Oligo)
	}

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

		ol.Slice(ppos, spos)
		if umap != nil {
			s := ol.String()
			if o, found := umap[s]; found {
				olcnt[o]++
			} else {
				umap[s] = ol
				olcnt[ol] = 1
				oligos = append(oligos, ol)
			}
		} else {
			oligos = append(oligos, ol)
		}
	}

	p.oligos = oligos
	p.olcnt = olcnt
}

func (p *Pool) Count(ol oligo.Oligo) int {
	if p.olcnt == nil {
		return 1
	}

	return p.olcnt[ol]
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

func TrimOligo(ol, prefix, suffix oligo.Oligo, dist int, keep bool) bool {
	var ppos, spos, plen, slen int

	if prefix != nil {
		ppos, plen = oligo.Find(ol, prefix, dist)
		if ppos < 0 {
			return false
		}
	}

	if suffix != nil {
		spos, slen = oligo.Find(ol, suffix, dist)
		if spos < 0 {
			return  false
		}
	} else {
		spos = ol.Len()
	}

	if !keep {
		ppos += plen
	} else {
		spos += slen
	}

	ol.Slice(ppos, spos)
	return true
}
