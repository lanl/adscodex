package moderate

import (
_	"fmt"
	"sort"
	"adscodex/oligo"
	"adscodex/oligo/short"
	"adscodex/errmdl"
)

type ModerateErrorModel struct {
	ins	[]float64	// probability of insertion error per nt
	del	[]float64	// probability of deletion error per nt
	sub	[][]float64	// probability of substitution error per nt (0 if the same nt)
//	noerr	float64		// probability of no error (1 - sum of the probabilities from all arrays)
}

func New(ins, del []float64, sub [][]float64) (em *ModerateErrorModel) {
	em = new(ModerateErrorModel)

	em.ins = ins
	em.del = del
	em.sub = sub
/*
	em.noerr = 1
	for _, v := range em.ins {
		em.noerr -= v
	}
	for _, v := range em.del {
		em.noerr -= v
	}
	for _, va := range em.sub {
		for _, v := range va {
			em.noerr -= v
		}
	}
*/

//	fmt.Printf("noerr %v\n", em.noerr)
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
