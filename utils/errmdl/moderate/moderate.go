package moderate

import (
_	"fmt"
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"sort"
	"sync"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/oligo/long"
	"adscodex/utils/errmdl"
)

type ModerateErrorDescr struct {
	Ins	[4]float64
	Del	[4]float64
	Sub	[4][4]float64
}

type ModerateErrorModel struct {
	sync.Mutex
	ins	[]float64	// probability of insertion error per nt
	del	[]float64	// probability of deletion error per nt
	sub	[][]float64	// probability of substitution error per nt (0 if the same nt)
//	noerr	float64		// probability of no error (1 - sum of the probabilities from all arrays)

	// some stats for oligo generation
	erri	float64		// total insertion error
//	errd	float64		// total deletion error
	
	// Oligo abundance error parameters
	// Negative binomial distributions
	p	float64

	// pseudorandom source
	rnd	*rand.Rand
}

func New(ins, del []float64, sub [][]float64, p float64, seed int64) (em *ModerateErrorModel) {
	em = new(ModerateErrorModel)

	em.ins = ins
	em.del = del
	em.sub = sub
	em.p = p
	em.rnd = rand.New(rand.NewSource(seed))

	for _, v := range ins {
		em.erri += v
	}

//	for _, v := range del {
//		em.errd += v
//	}

	// Experiment
//	for i := 0; i < len(em.del); i++ {
//		em.del[i] *= 4
//	}
/*
	for i := 0; i < len(em.sub); i++ {
		var e float64

		for j := 0; j < len(em.sub[i]); j++ {
			if i == j {
				continue
			}

			em.sub[i][j] *= 4
			e += em.sub[i][j]
		}
		em.sub[i][i] = 1 - e
	}
*/

	return
}

func FromJson(fname string, p float64, seed int64) (em *ModerateErrorModel, err error) {
	var ed ModerateErrorDescr
	var b []byte

	b, err = ioutil.ReadFile(fname)
	if err != nil {
		return
	}

	err = json.Unmarshal(b, &ed)
	if err != nil {
		return
	}

	sub := make([][]float64, 4)
	sub[0] = ed.Sub[0][:]
	sub[1] = ed.Sub[1][:]
	sub[2] = ed.Sub[2][:]
	sub[3] = ed.Sub[3][:]
	em = New(ed.Ins[:], ed.Del[:], sub, p, seed)
	return
}

func (em *ModerateErrorModel) SortedErrors(ol oligo.Oligo, minprob float64) (ret []errmdl.OligoProb) {
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
			ols[n].Ol, _ = short.FromString(s)
			ols[n].Prob = p
			n++
		}
	}

	sort.Slice(ols, func(i, j int) bool {
		return ols[i].Prob > ols[j].Prob
	})

	return ols
}

func (em *ModerateErrorModel) genErrors(prefix, suffix string, err, minerr float64, m map[string] float64) {
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
//	em.genErrors(prefix + string(suffix[0]), suffix[1:], err * em.noerr, minerr, m)

	// insertion errors
//	fmt.Printf("%s--- insertions\n", ind)
	for n := 0; n < 4; n++ {
		em.genErrors(prefix + oligo.Nt2String(n), suffix, err * em.ins[n], minerr, m)
	}

	// deletion error
	n := oligo.String2Nt(string(suffix[0]))
	em.genErrors(prefix, suffix[1:], err * em.del[n], minerr, m)

	// substitution errors
//	fmt.Printf("%s--- substitutions\n", ind)
	from := oligo.String2Nt(string(suffix[0]))
	suff := suffix[1:]
	for n := 0; n < 4; n++ {
		e := err * em.sub[from][n]
		if e == 0 {
			continue
		}

		to := oligo.Nt2String(n)[0]
		em.genErrors(prefix + string(to), suff, e, minerr, m)
	}
}

func (em  *ModerateErrorModel) GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()

	em.Lock()
	seq, errnum = em.genSeq(seq)
	em.Unlock()

	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em  *ModerateErrorModel) genOneLocked(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()
	seq, errnum = em.genSeq(seq)
	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em *ModerateErrorModel) genSeq(seq string) (ret string, errnum int) {
	for i := 0; i < len(seq); i++ {
		nt := oligo.Char2Nt(seq[i])
		p := em.rnd.Float64()
		pp := p * (1 + em.erri + em.del[nt])	// scale to total error prob
		if pp < em.erri {
			// insertion
			for j := 0; j < 4; j++ {
				pp -= em.ins[j]
				if pp < 0 || j == 3 {
					ret += oligo.Nt2String(j)
					break
				}
			}

			ret += string(nt)
		} else if pp < em.erri + em.del[nt] {
			// deletion
		} else {
			// substitution
			s := em.sub[nt]
			for j := 0; j < 4; j++ {
				p -= s[j]
				if p < 0 || j == 3 {
					ret += oligo.Nt2String(j)
					// fix errnum if it's a noop
					if j == int(nt) {
						errnum--
					}
					break
				}
			}
		}

		errnum++
	}

	return seq, errnum
}

func (em  *ModerateErrorModel) GenMany(numreads int, ols []oligo.Oligo) (rs []oligo.Oligo, errnum int) {

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

