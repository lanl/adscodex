// The oligo package defines oligo sequence data structures and functions
package oligo

const (
	A = 0
	T = 1
	C = 2
	G = 3
)

// Generic interface of an oligo that represents an oligo sequence.
// The actual implementations are in packages short and long
type Oligo interface {
	// Length of the oligo
	Len() int

	// Converts the oligo to string
	String() string

	// Compares two oligos. 
	// Returns
	//     -1 if the oligo comes before the other oligo
	//     0 if the oligos are the same
	//     1 if the oligo comes after the other oligo
	// Note: if the oligos are of different lengths, the shorter one comes
	// before the longer one
	Cmp(other Oligo) int

	// Moves to the next oligo
	// Returns false if it reaches the limit (and doesn't change the current oligy)
	Next() bool

	// Returns the nucleotide at position idx, -1 if out of bounds
	At(idx int) int

	// Returns part of the oligo
	Slice(start, end int) Oligo

	// Creates a copy of the oligo
	Clone() Oligo

	// Appends the specified oligo
	// Returns false if error (the resulting oligo too big)
	Append(other Oligo) bool
}

var ntNames = "ATCG"

// Converts an numeric value of a nucleotide (nt) to its string value
func Nt2String(nt int) string {
	if nt<0 || nt > len(ntNames) {
		return "?"
	}

	return string(ntNames[nt])
}

// Converts string value of a nt to its numeric value
func String2Nt(nt string) int {
	switch nt {
	default:
		return -1
	case "A":
		return A
	case "T":
		return T
	case "C":
		return C
	case "G":
		return G
	}
}

// Calculates the GC content of an oligo. 
// Returns a value between 0 (no GC) and 1.
func GCcontent(o Oligo) float64 {
	var n int

	for i := 0; i < o.Len(); i++ {
		nt := o.At(i)
		if nt == C || nt == G {
			n++
		}
	}

	return float64(n)/float64(o.Len())
}

// Implements Levenshtein distance
func Distance(a, b Oligo) int {
	f := make([]int, b.Len() + 1)

	for j := range f {
		f[j] = j
	}

	for n := 0; n < a.Len(); n++ {
		ca := a.At(n)
		j := 1
		fj1 := f[0] // fj1 is the value of f[j - 1] in last iteration
		f[0]++
		for m := 0; m < b.Len(); m++ {
			cb := b.At(m)
			mn := min(f[j]+1, f[j-1]+1) // delete & insert
			if cb != ca {
				mn = min(mn, fj1+1) // change
			} else {
				mn = min(mn, fj1) // matched
			}

			fj1, f[j] = f[j], mn // save f[j] to fj1(j is about to increase), update f[j] to mn
			j++
		}
	}

	return f[len(f)-1]
}

// Finds subsequence in a sequence, with up to maxdist errors allowed.
// Similar to Levenshtein distance.
// Returns the position and the length in the original sequence, -1 for position
// if not found.
func Find(s, subseq Oligo, maxdist int) (pos int, length int) {
	slen := s.Len()
	sslen := subseq.Len()
	f := make([]int, slen + 1)
	l := make([]int, slen + 1)
	for i := range f {
		f[i] = 0
		l[i] = 0
	}

	for i := 0; i < sslen; i++ {
		ca := subseq.At(i)
		fj1 := f[0] // fj1 is the value of f[j - 1] in last iteration
		lj1 := l[0]
		f[0]++
		l[0]++
		mdist := f[0]
		for j := 0; j < slen; j++ {
			cb := s.At(j)

			mn, ln := min2(f[j+1]+1, f[j]+1, l[j+1]-1, l[j]+1) // delete & insert
			if cb != ca {
				mn, ln = min2(mn, fj1+1, ln, lj1) // change
			} else {
				mn, ln = min2(mn, fj1, ln, lj1) // matched
			}

			fj1, f[j+1] = f[j+1], mn // save f[j] to fj1(j is about to increase), update f[j] to mn
			lj1, l[j+1] = l[j+1], ln

			if f[j+1] < mdist {
				mdist = f[j+1]
			}
		}

		if mdist > maxdist {
			return -1, 0
		}
	}

	end := len(f) - 1
	minval := f[end]
	for i := end - 1; i >= 0; i-- {
		if minval > f[i] {
			minval = f[i]
			end = i
		}
	}

	length = sslen + l[end]
	pos = end - sslen - l[end]

	return
}

// Returns true if the seq starts with prefix (with up to maxdist errors)
func HasPrefix(seq, prefix Oligo, maxdist int) bool {
	p, _ := Find(seq, prefix, maxdist)

	return p == 0
}

// Returns true if the sequence ends with the suffix (with up to maxdist errors)
func HasSuffix(seq, suffix Oligo, maxdist int) bool {
	// since Find finds the first occurence, we might need to call it few times
	for seq.Len() > suffix.Len() {
		p, l := Find(seq, suffix, maxdist)
		// no match, faill
		if p < 0 {
			return false
		}

		// if we matched everything until the end, success
		if p + l == seq.Len() {
			return true
		}

		// cut what we matched and try again
		seq = seq.Slice(p + l, -1)
	}

	return false
}

func Diff(from, to Oligo) (int, string) {
	m := from.Len()
	n := to.Len()

	v := make([][]int, m + 1)
	b := make([][]string, m + 1)
	b[0] = make([]string, n + 1)
	v[0] = make([]int, n + 1)
	for i:=0; i < n+1; i++ {
		v[0][i] = i
		b[0][i] = "I"
	}

	for i:= 0; i < m; i++ {
		v[i+1] = make([]int, n + 1)
		b[i+1] = make([]string, n + 1)
		v[i+1][0] = i + 1
		b[i+1][0] = "D"
		for j := 0; j < n; j++ {
			deletionCost := v[i][j+1] + 1
			insertionCost := v[i+1][j] + 1
			substitutionCost := v[i][j]
			b[i+1][j+1] = "R"
			if from.At(i) != to.At(j) {
				substitutionCost++
			}

			mincost := substitutionCost
			if mincost > insertionCost {
				mincost = insertionCost
				b[i+1][j+1] = "I"
			}
			if mincost > deletionCost {
				mincost = deletionCost
				b[i+1][j+1] = "D"
			}

			v[i+1][j+1] = mincost
				
		}

	}

	// now backtrack to get the actions
	diff := ""
	for i, j := m, n; i > 0 || j > 0; {
		switch b[i][j] {
		case "D":
			diff = "D" + diff
			i--

		case "R":
			if i>0 && j>0 && v[i-1][j-1] != v[i][j] {
				diff = "R" + diff
			} else {
				diff = "-" + diff
			}
			i--
			j--

		case "I":
			diff = "I" + diff
			j--
		}
	}

	return v[m][n], diff
}

func min(a, b int) int {
         if a <= b {
                 return a
         } else {
                 return b
         }
}

func min2(a, b, aa, bb int) (int, int) {
	if a <= b {
		return a, aa
	} else {
		return b, bb
	}
}
