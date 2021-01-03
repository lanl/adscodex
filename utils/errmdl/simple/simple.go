package simple

import (
	"math/rand"
	"sync"
	"acoma/oligo"
	"acoma/oligo/long"
_	"acoma/utils/errmdl"
)

type SimpleErrorModel struct {
	sync.Mutex

	// Single oligo error parameters
	erri	float64		// probability of insertion error
	errdi	float64		// probability of insertion or deletion error (only deletion error is errdi - erri)
	err	float64		// total probability of error per position (only substitution error is err - errdi - erri)

	// Oligo abundance error parameters
	// Negative binomial distributions
	p	float64

	// pseudorandom source
	rnd	*rand.Rand
}

func New(ierr, derr, serr, p float64, seed int64) (em *SimpleErrorModel) {
	em = new(SimpleErrorModel)

	em.erri = ierr
	em.errdi = ierr + derr
	em.err = ierr + derr + serr
	em.p = p

	em.rnd = rand.New(rand.NewSource(seed))
	return
}

func (em  *SimpleErrorModel) GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()

	em.Lock()
	seq, errnum = em.genSeq(seq)
	em.Unlock()

	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em  *SimpleErrorModel) genOneLocked(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()
	seq, errnum = em.genSeq(seq)
	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em *SimpleErrorModel) genSeq(seq string) (ret string, errnum int) {
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

func (em  *SimpleErrorModel) GenMany(numreads int, ols []oligo.Oligo) (rs []oligo.Oligo, errnum int) {

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
