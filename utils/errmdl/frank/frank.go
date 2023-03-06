package simple

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/utils/errmdl"
)

type FrankErrorModel struct {
	sync.Mutex

	p0	float64
	alpha	float64
	q	float64

	// pseudorandom source
	rnd	*rand.Rand
}

func NewFrankErrorModel(p0, alpha, q float64, seed int64) (em *FrankErrorModel) {
	em = new(FrankErrorModel)

	em.p0 = p0
	em.alpha = alpha
	em.q = q

	em.rnd = rand.New(rand.NewSource(seed))
	return
}

func (em *FrankErrorModel) bernoulli(p float64) float64 {
	rnd := em.rnd.Float64()
	if rnd < p {
		return 1
	} else {
		return 0
	}
}

func (em *FrankErrorModel) errnum(olen int) (n int) {
	var p float64

	if bernoulli(em.alpha) == 1 {
		p = em.p0
	} else {
		pi := ???
	}
	for i := 0; i < n; i++ {
		n += bernoulli(em.p0)
	}
		
}

func (em  *FrankErrorModel) GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()

	em.Lock()
	seq, errnum = em.genSeq(seq)
	em.Unlock()

	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em  *FrankErrorModel) genOneLocked(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()
	seq, errnum = em.genSeq(seq)
	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em *FrankErrorModel) genSeq(seq string) (ret string, errnum int) {
	for i := 0; i < len(seq); i++ {
		p := em.rnd.Float64()
		if p > em.err {
			continue
		}

		if p < em.erri {
			// insertion
			seq = seq[0:i] + oligo.Nt2String(em.rnd.Intn(4)) + seq[i:]
			i++
		} else if p < em.errdi {
			// deletion
			if i+1 < len(seq) {
				seq = seq[0:i] + seq[i+1:]
			} else {
				seq = seq[0:i]
			}
			i--
		} else {
			// substitution
			var r string

			if i+1 < len(seq) {
				r = seq[i+1:]
			}

			n := em.rnd.Intn(3)
			if n >= oligo.Char2Nt(seq[i]) {
				n++
			}

			seq = seq[0:i] + oligo.Nt2String(n) + r
		}

		errnum++
	}

	return seq, errnum
}

func (em  *FrankErrorModel) GenMany(numreads int, ols []oligo.Oligo) (rs []oligo.Oligo, errnum int) {

	// first shuffle them so if numreads is low, the order doesn't affect the outcome much
	em.Lock()
	defer em.Unlock()

	em.rnd.Shuffle(len(ols),  func (i, j int) {
		ols[i], ols[j] = ols[j], ols[i]
	})

	n := int((float64(numreads)/float64(len(ols))) * ((1 - em.p)/em.p))
	for _, o := range ols {
		// calculate the binomial distribution for the oligo
		var count int

		for i := 0; i <= n; count++ {
			if em.rnd.Float64() > em.p {
				i++
			}
		}
		count -= n

		// generate the reads for the oligo
		for i := 0; i < count; i++ {
			r, en := em.genOneLocked(o)
			rs = append(rs, r)
			errnum += en
		}
	}

	em.rnd.Shuffle(len(rs),  func (i, j int) {
		rs[i], rs[j] = rs[j], rs[i]
	})

	if len(rs) > numreads {
		rs = rs[0:numreads]
	}
/*
	for i := 0; i < numreads; i++ {
		o := ols[em.rnd.Int31n(int32(len(ols)))]
		r, en := em.genOneLocked(o)
		rs = append(rs, r)
		errnum += en
	}
*/
	return

}

func (em *FrankErrorModel) SortedErrors(ol oligo.Oligo, minprob float64) (ret []errmdl.OligoProb) {
	fmt.Printf("Sorted Errors: insertion %v deletion %v substitutions %v\n", em.erri, em.errdi - em.erri, em.err - em.errdi)
	m := make(map[string] float64)

	seq := ol.String()
	em.genErrors("", seq, 1, minprob, m)

	n := 0
	for _, p := range m {
		if p >= minprob {
			n++
		}
	}

	ols := make([]errmdl.OligoProb, n)
	n = 0
	for s, p := range(m) {
		if p >= minprob {
			ols[n].Ol, _ = long.FromString(s)
			ols[n].Prob = p
			n++
		}
	}

	sort.Slice(ols, func(i, j int) bool {
		return ols[i].Prob > ols[j].Prob
	})

	return ols
}

func (em *FrankErrorModel) genErrors(prefix, suffix string, err, minerr float64, m map[string] float64) {
//	ind := ""
//	for i := 0; i < len(prefix); i++ {
//		ind += " ";
//	}

	if err < minerr {
//		fmt.Printf("%s< %v %v\n", ind, err, minerr)
		return
	}

	if len(suffix) == 0 {
//		fmt.Printf("%s= %v err %v\n", ind, prefix, err)
		m[prefix] += err
		return
	}

//	fmt.Printf("%sprefix %v suffix %v err %v\n", ind, prefix, suffix, err)
	// no error
//	fmt.Printf("%s--- no errors\n", ind)
	em.genErrors(prefix + string(suffix[0]), suffix[1:], err * (1 - em.err), minerr, m)

	// insertion errors
//	fmt.Printf("%s--- insertions\n", ind)
	e := err * (em.erri/4)
	em.genErrors(prefix + string('A'), suffix, e, minerr, m)
	em.genErrors(prefix + string('T'), suffix, e, minerr, m)
	em.genErrors(prefix + string('C'), suffix, e, minerr, m)
	em.genErrors(prefix + string('G'), suffix, e, minerr, m)

	// deletion error
//	fmt.Printf("%s--- deletions\n", ind)
	e = err * (em.errdi - em.erri)
	em.genErrors(prefix, suffix[1:], e, minerr, m)

	// substitusion errors
//	fmt.Printf("%s--- substitusions\n", ind)
	e = err * ((em.err - em.errdi)/4)
	s := suffix[0]
	suff := suffix[1:]

	for n := 0; n < 4; n++ {
		nt := oligo.Nt2String(n)[0]
		if nt != s {
			em.genErrors(prefix + string(nt), suff, e, minerr, m)
		}
	}
}
