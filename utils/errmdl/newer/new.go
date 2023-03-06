package newer

import (
	"fmt"
	"encoding/json"
	"io/ioutil"
	"math"
	"math/rand"
_	"sort"
	"sync"
	"adscodex/oligo"
_	"adscodex/oligo/short"
	"adscodex/oligo/long"
_	"adscodex/utils/errmdl"
)

type NewErrorDescr struct {
	OLen	int		// oligo length
	Dropout	float64		// oligo dropout rate

	InsErr	float64
	SubErr	float64
	DelErr	float64

	InsDist	[]float64
	SubDist	[]float64
	DelDist	[]float64
}

type Dist struct {
	p	float64		// probability
	olen	int		// oligo length
	pdist	[]float64	// probability per number of errors
	cs	[]float64	// cumulative area until this interval
	a	[]float64	// slope for each interval
}

type NewErrorModel struct {
	sync.Mutex
	idist	*Dist
	sdist	*Dist
	ddist	*Dist
	iscale	float64
	sscale	float64
	dscale	float64

	dropout	float64		// oligo dropout rate
	olen	int		// oligo length

	// pseudorandom source
	seed	int64
	rnd	*rand.Rand
}

func newDist(p float64, dist []float64, olen int) (ret *Dist) {
//	fmt.Printf("newDist p %v\n", p)
	ret = new(Dist)
	ret.p = p
	ret.pdist = make([]float64, len(dist) + 1)
	ret.cs = make([]float64, len(dist) + 1)
	ret.a = make([]float64, len(dist) + 1)

	// setup the distributions with adjustments
	ret.pdist[len(dist)] = 0 // dist[len(dist) - 1] / 2
	var e float64
	for i := len(dist) - 1; i >= 0; i-- {
		e += float64(i) * dist[i] * (100/float64(olen))
		ret.pdist[i] = 2*dist[i] - ret.pdist[i+1]
	}

	fmt.Printf("newDist p %v e %v\n", p, e/float64(len(dist)))
	// FIXME: not sure this is right...
	ret.p =  e/float64(len(dist))

/*
	for i := 0; i < len(dist); i++ {
		ret.pdist[i] = dist[i]
	}

	for i := 0; i < len(dist); i++ {
		var nv float64

		v := ret.pdist[i]
		nv = ret.pdist[i+1]

		x := (v - nv) / 3
		ret.pdist[i] += x
		ret.pdist[i+1] += 2*x
	}
*/

	// calculate areas and slopes
	var cs float64
	for i := 0; i < len(ret.pdist); i++ {
		var nv float64

		v := ret.pdist[i]
		if i+1 < len(ret.pdist) {
			nv = ret.pdist[i+1]
		}

		cs += (v + nv)/2
		ret.cs[i] = cs
		ret.a[i] = nv - v
//		fmt.Printf("\t%d\tp %v\tcs %v\ta %v\n", i, v, cs, ret.a[i])
	}

//	fmt.Printf("\tadj %v cs %v\n", adj, cs)
	return
}

func (d *Dist) randomError(rnd *rand.Rand) float64 {
	s := rnd.Float64()
	n := 0
	for ; n < len(d.pdist); n++ {
		if s < d.cs[n] {
			break
		}
	}

//	fmt.Printf("randomError (%v) s %v n %d\n", d.p, s, n)

	// calculate float64 error with slope
	ss := s
	if n > 0 {
		ss -= d.cs[n-1]
	}

//	fmt.Printf("\tss %v a %v\n", ss, d.a[n])

	a := d.a[n]
	b := 2 * d.pdist[n]
	c := -2 * ss
	x := (-b + math.Sqrt(b*b - 4*a*c))/(2*a)
//	fmt.Printf("\ta %v b %v c %v x %v | %v\n", a, b, c, x, b*b - 4*a*c)
	return float64(n) + x
}
func New(ierr, serr, derr float64, ins, sub, del []float64, dropout float64, olen int, seed int64) (em *NewErrorModel) {
	em = new(NewErrorModel)

	em.idist = newDist(ierr, ins, olen)
	em.sdist = newDist(serr, sub, olen)
	em.ddist = newDist(derr, del, olen)
	em.iscale = 1
	em.sscale = 1
	em.dscale = 1
	em.dropout = dropout
	em.olen = olen
	em.seed = seed
	em.rnd = rand.New(rand.NewSource(seed))

	return
}

func FromJson(fname string, seed int64) (em *NewErrorModel, err error) {
	var ed NewErrorDescr
	var b []byte

	b, err = ioutil.ReadFile(fname)
	if err != nil {
		return
	}

	err = json.Unmarshal(b, &ed)
	if err != nil {
		return
	}

	em = New(ed.InsErr, ed.SubErr, ed.DelErr, ed.InsDist[:], ed.SubDist[:], ed.DelDist[:], ed.Dropout, ed.OLen, seed)
	return
}

func (em *NewErrorModel) Scale(ierr, serr, derr float64) {
	if ierr > 0 {
		em.iscale = ierr / em.idist.p
	}

	if serr > 0 {
		em.sscale = serr / em.sdist.p
	}

	if derr > 0 {
		em.dscale = derr / em.ddist.p
	}
}	

func (em *NewErrorModel) rndErrors(d *Dist, scale float64, olen int) int {

	r := d.randomError(em.rnd)

	r *= scale
	n := (r * float64(olen)) / float64(em.olen)

//	fmt.Printf("\tr %v scale %v n %v\n", r, scale, n)
	return int(n)
}

func (em  *NewErrorModel) GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()

	em.Lock()
	seq, errnum = em.genSeq(seq)
	em.Unlock()

	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em  *NewErrorModel) genOneLocked(ol oligo.Oligo) (r oligo.Oligo, errnum int) {
	seq := ol.String()
	seq, errnum = em.genSeq(seq)
	r, _ = long.FromString(seq)
	return
}

// the random source needs to be locked
func (em *NewErrorModel) genSeq(seq string) (ret string, errnum int) {
	n := 0
again:
	inum := em.rndErrors(em.idist, em.iscale, len(seq))
	snum := em.rndErrors(em.sdist, em.sscale, len(seq))
	dnum := em.rndErrors(em.ddist, em.dscale, len(seq))

//	fmt.Printf("errors %d %d %d %d\n", inum, snum, dnum, len(seq))
	nlen := len(seq) + inum - dnum
	// we can't put more errors than the length of the oligo
	if inum+snum+dnum >= nlen {
		if n > 100 {
			panic(fmt.Sprintf("something wrong %d %d %d", inum, snum, dnum))
		}
		n++
		goto again
	}

	a := make([]byte, nlen)

	// insertion errors
	for i := 0; i < inum; {
		p := em.rnd.Intn(nlen)
		if a[p] != 0 {
			continue
		}

		a[p] = 'I' // oligo.Nt2Char(em.rnd.Int31(4))
		i++
	}

	// deletion errors
	for i := 0; i < dnum; {
		p := em.rnd.Intn(nlen)
		if a[p] != 0 {
			continue
		}

		a[p] = 'D'
		i++
	}

	// substitution errors
	for i := 0; i < snum; {
		p := em.rnd.Intn(nlen)
		if a[p] != 0 {
			continue
		}

		a[p] = 'S'
		i++
	}

	// now create the actual sequence
	for i, j := 0, 0; j < len(a); j++ {
		c := a[j]
		switch c {
		case 0:
			ret += string(seq[i])
			i++

		case 'I':
			ret += oligo.Nt2String(em.rnd.Intn(4))

		case 'D':
			i++

		case 'S':
			n := em.rnd.Intn(3)
			nt := oligo.Char2Nt(seq[i])
			for m := 0; m < 4; m++ {
				if n == 0 {
					ret += oligo.Nt2String(m)
					break
				}

				if m != nt {
					n--
				}
			}
		}
	}

//	fmt.Printf(">>> %v\n", ret)
	errnum = inum + snum + dnum
	return
}

func (em  *NewErrorModel) GenMany(numreads int, ols []oligo.Oligo) (rs []oligo.Oligo, errnum int) {
	// first shuffle them so if numreads is low, the order doesn't affect the outcome much
	em.Lock()
	defer em.Unlock()

	em.rnd.Shuffle(len(ols),  func (i, j int) {
		ols[i], ols[j] = ols[j], ols[i]
	})

	// remove a number of oligos as per dropout rate
	ols = ols[0:len(ols) - int(em.dropout * float64(len(ols)))]

	for i := 0; i < numreads; i++ {
		idx := em.rnd.Intn(len(ols))
		r, en := em.genOneLocked(ols[idx])
		rs = append(rs, r)
		errnum += en
	}

	if len(rs) > numreads {
		rs = rs[0:numreads]
	}
	return

}

func (em *NewErrorModel) String() (ret string) {
	ret = fmt.Sprintf("Rates (%v, %v, %v)\n", em.idist.p, em.sdist.p, em.ddist.p)
	ret = fmt.Sprintf("Olen %d\n", em.olen)
	ret += fmt.Sprintf("Ins Distribution\n")
	for i, v := range em.idist.pdist {
		ret += fmt.Sprintf("\t%d\t%v\n", i, v)
	}

	ret += fmt.Sprintf("Sub Distribution\n")
	for i, v := range em.sdist.pdist {
		ret += fmt.Sprintf("\t%d\t%v\n", i, v)
	}

	ret += fmt.Sprintf("Del Distribution\n")
	for i, v := range em.ddist.pdist {
		ret += fmt.Sprintf("\t%d\t%v\n", i, v)
	}

	return
}
