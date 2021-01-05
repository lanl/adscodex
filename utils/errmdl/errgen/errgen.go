package errgen

import (
_	"errors"
_	"fmt"
	"math/rand"
	"sync"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/utils/match/file"
)

type ErrgenErrorModel struct {
	sync.Mutex
	matches [][]*file.Match
	seqs	[]*file.Match
	rnd	*rand.Rand
}

func New(fname string, seed int64) (em *ErrgenErrorModel, err error) {
	em = new(ErrgenErrorModel)
	em.matches, err = file.Read(fname)
	if err != nil {
		em = nil
		return
	}

	for _, mm := range em.matches {
		em.seqs = append(em.seqs, mm...)
	}

	em.rnd = rand.New(rand.NewSource(seed))
	return
}

func (em *ErrgenErrorModel) GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	em.Lock()
	r, errnum = em.genOneLocked(ol)
	em.Unlock()

	return
}

func (em  *ErrgenErrorModel) GenMany(numreads int, ols []oligo.Oligo) (rs []oligo.Oligo, errnum int) {
	em.Lock()
	for i := 0; i < numreads; i++ {
		n := em.rnd.Int31n(int32(len(ols)))
		r, en := em.genOneLocked(ols[n])
		rs = append(rs, r)
		errnum += en
	}
	em.Unlock()

	return
}

func (em *ErrgenErrorModel) genOneLocked(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	actions := em.seqs[em.rnd.Int31n(int32(len(em.seqs)))].Diff
	sol := ol.String()

	var ret string
	for ai, oi := 0, 0; oi < len(sol) && ai < len(actions); ai++ {
		a := actions[ai]

		switch a {
		case '-':
			ret += string(sol[oi])
			oi++

		case 'D':
			oi++
			errnum++

		case 'I':
			ret += oligo.Nt2String(int(rand.Int31n(4)))
			errnum++

		case 'A', 'T', 'C', 'G':
			ret += string(a)
			errnum++

		case 'R':
			for {
				nt := oligo.Nt2String(int(rand.Int31n(4)))
				if sol[oi] != nt[0] {
					ret += nt
					break
				}
			}
			errnum++
			oi++
		}
	}

	r = long.FromString1(ret)

	return
}
