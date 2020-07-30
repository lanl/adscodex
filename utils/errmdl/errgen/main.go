package main

import (
	"bufio"
_	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"acoma/oligo"
	"acoma/io/csv"
	"acoma/utils"
)

var gennum = flag.Int("n", 0, "number of sequences to generate")
var errfname = flag.String("e", "", "error model file")
var matchseed = flag.Int64("ms", 0, "seed for the random generator used for the matches")
var dataseed = flag.Int64("ds", 0, "seed for the random generator used for the data")
var dsfname = flag.String("ds", "", "synthesis dataset")
var unique = flag.Bool("u", true, "print only unique sequences")
var printOrig = flag.Bool("p", false, "print the original oligo")
var model = flag.Int("m", 1, "model")

var matches [][]string

func readErrorModel(fname string) (err error) {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()

	ms := make(map[int][]string)
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		var id, count int

		line := sc.Text()
		if line == "" {
			continue
		}

		ls := strings.Split(line, " ")
		if len(ls) == 1 {
			// support both space-separated and comma-separated
			ls = strings.Split(line, ",")
		}

		if n, err := strconv.ParseUint(ls[0], 10, 32); err != nil {
			return err
		} else {
			id = int(n)
		}
		
		if len(ls) < 2 {
			return fmt.Errorf("invalid line: %d '%s'", n, sc.Text())
		}

		if n, err := strconv.ParseUint(ls[1], 10, 32); err != nil {
			return err
		} else {
			count = int(n)
		}

		if count != 0 && len(ls) < 3 {
			return fmt.Errorf("invalid line: %d '%s'", n, sc.Text())
		}

		ms[id] = nil
		for i := 0; i < int(n); i++ {
			ms[id] = append(ms[id], ls[2])
		}

		n++
	}

	for _, m := range ms {
		matches = append(matches, m)
	}

//	fmt.Printf("%d IDs\n", len(matches))
	return nil
}

// generate sequence by mimicking the errors from the match
func genSeq(actions string, ol oligo.Oligo) (ret string) {
	// don't stretch or shrink
//	fmt.Printf("%v\n", actions)
//	fmt.Printf("%v\n", oligo)
	sol := ol.String()
	for ai, oi := 0, 0; oi < len(sol) && ai < len(actions); ai++ {
		a := actions[ai]

		switch a {
		case '-':
			ret += string(sol[oi])
			oi++

		case 'D':
			oi++

		case 'I':
			ret += oligo.Nt2String(int(rand.Int31n(4)))

		case 'A', 'T', 'C', 'G':
			ret += string(a)

		case 'R':
			for {
				nt := oligo.Nt2String(int(rand.Int31n(4)))
				if sol[oi] != nt[0] {
					ret += nt
					break
				}
			}
			oi++
		}
	}

	return
}

func main() {
	flag.Parse()

	if *gennum == 0 {
		fmt.Printf("Expecting number of sequences to generate\n")
		return
	}

	if *errfname == "" {
		fmt.Printf("Expecting error model file name\n")
		return
	}

	if *dsfname == "" {
		fmt.Printf("Expecting dataset file name\n")
		return
	}

	mseed := *matchseed
	if mseed == 0 {
		mseed = time.Now().UnixNano()
	}

	dseed := *dataseed
	if dseed == 0 {
		dseed = time.Now().Unix()
	}

	// read the error model
	if err := readErrorModel(*errfname); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// read the dataset
	dspool, err := utils.ReadPool([]string { *dsfname }, false, csv.Parse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	oligos := dspool.Oligos()

	mrnd := rand.New(rand.NewSource(mseed))
	drnd := rand.New(rand.NewSource(dseed))
	nmatches := int32(len(matches))
	noligos := int32(len(oligos))

	var omap map[int] int
	switch (*model) {
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid model: %d\n", *model)
	case 1:
		// nothing

	case 2:
		omap = make(map[int] int)
		for n, _ := range oligos {
			omap[n] = int(mrnd.Int31n(nmatches))
		}
	}

	// generate the results based on the error model
	uqmap := make(map[string] bool)
	for i := 0; i < *gennum;  {
		var midx int

		didx := int(drnd.Int31n(noligos))
		if omap != nil {
			midx = omap[didx]
		} else {
			midx = int(mrnd.Int31n(nmatches))
		}

		// get random diff from the list
		m := matches[int(midx)]
		if len(m) == 0 {
			// no matches, drop
			mrnd.Int31n(5)
			continue
		}

		midx = int(mrnd.Int31n(int32(len(m))))
		seq := genSeq(m[midx], oligos[didx])
		if !*unique || !uqmap[seq] {
			if *printOrig {
				fmt.Printf("%v %v\n", seq, oligos[didx])
			} else {
				fmt.Printf("%v\n", seq)
			}

			uqmap[seq] = true
			i++
		}
	}

	return
}
