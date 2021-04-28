package file

import (
	"bufio"
	"compress/gzip"
_	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"adscodex/oligo"
	"adscodex/oligo/long"
_	"adscodex/utils"
)

type Match struct {
	Id	int
	Orig	oligo.Oligo	// original oligo from the dataset
	Read	oligo.Oligo	// the read that is matched to it
	Count	int		// number of reads
	Cubu	float64		// read cubundance
	Diff	string		// difference between original and the read
}

// Reads a match file.
// Returns an array, one element per original oligo from the dataset.
// The element itself is an array of matches. Depending on the parameters
// used when the file was created, some elements in the struct may not be populated.
func Read(fname string) (matches [][]*Match, err error) {
	ms := make(map[int][]*Match)
	maxid := 0
	err = Parse(fname, func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
		m := new(Match)
		m.Id = id
		m.Count = count
		m.Cubu = cubu
		m.Diff = diff
		m.Orig = orig
		m.Read = read

		ms[id] = append(ms[id], m)
		if maxid < id {
			maxid = id
		}
	})

	if err != nil {
		return
	}

	matches = make([][]*Match, maxid + 1)
	for id, mm := range ms {
		matches[id] = mm
	}

	return
}

func Parse(fname string, process func(id, count int, diff string, cubu float64, orig, read oligo.Oligo)) (err error) {
	var r io.Reader

	f, e := os.Open(fname)
	if e != nil {
		err = e
		return
	}
	defer f.Close()

	if cf, err := gzip.NewReader(f); err == nil {
		r = cf
	} else {
		r = f
		f.Seek(0, 0)
	}

	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		var id, count int
		var diff, sorig, sread string
		var cubu float64
		var orig, read oligo.Oligo

		line := sc.Text()
		if line == "" {
			continue
		}

		ls := strings.Split(line, " ")
		if len(ls) == 1 {
			// support both space-separated and comma-separated
			ls = strings.Split(line, ",")
		}

		if nn, e := strconv.ParseUint(ls[0], 10, 32); e != nil {
			err = fmt.Errorf("invalid line: %d: %v\n", n, line)
			return
		} else {
			id = int(nn)
		}

		if len(ls) < 2 {
			err = fmt.Errorf("invalid line: %d '%s'", n, line)
			return
		}

		if n, e := strconv.ParseUint(ls[1], 10, 32); e != nil {
			err = e
			return
		} else {
			count = int(n)
		}

		if count == 0 {
			// there were no matches for the original sequence
			switch len(ls) {
			default:
				err = fmt.Errorf("invalid line: %d '%s'", n, line)
				return

			case 2:
				// nothing more to parse

			case 3:
				sorig = ls[2]
			}
		} else {
			// the original sequence was matched
			if len(ls) != 4 && len(ls) != 6 {
				err = fmt.Errorf("invalid line: %d '%s'", n, line)
				return
			}

			diff = ls[2]
			if v, e := strconv.ParseFloat(ls[3], 64); err != nil {
				err = e
				return
			} else {
				cubu = v
			}

			if len(ls) > 4 {
				sorig = ls[4]
				sread = ls[5]
			}
		}

		if o, ok := long.FromString(sorig); !ok {
			err  = fmt.Errorf("invalid oligo: %s\n", sorig)
			return
		} else {
			orig = o
		}

		if o, ok := long.FromString(sread); !ok {
			err  = fmt.Errorf("invalid oligo: %s\n", sread)
			return
		} else {
			read = o
		}

		process(id, count, diff, cubu, orig, read)
		n++
	}
	return
}

func ParseParallel(fname string, numprocs int, process func(id, count int, diff string, cubu float64, orig, read oligo.Oligo)) (err error) {
	if numprocs == 0 {
		numprocs = runtime.NumCPU()
	}

	ch := make(chan *Match)
	// start up the goroutines
	for i := 0; i < numprocs; i++ {
		go func() {
			for  {
				m := <-ch
				if m == nil {
					return
				}

				process(m.Id, m.Count, m.Diff, m.Cubu, m.Orig, m.Read)
			}
		}()
	}

	err = Parse(fname, func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
		ch <- &Match{id, orig, read, count, cubu, diff}
	})

	// wind down the goroutines
	for i := 0; i < numprocs; i++ {
		ch <- nil
	}

	return
}
