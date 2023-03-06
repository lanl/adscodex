package csv

import (
        "bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"adscodex/oligo"
	"adscodex/oligo/long"
)

func Read(fname string, ignoreBad bool) ([]oligo.Oligo, error) {
	var oligos []oligo.Oligo

	err := Parse(fname, func(id, sequence string, quality []byte, reverse bool) error {
		ol, ok := long.FromString(sequence)
		if !ok {
			if ignoreBad {
				// skip
				return nil
			} else {
				return fmt.Errorf("invalid oligo: %s\n", sequence)
			}
		}

		if reverse {
			oligo.Reverse(ol)
			oligo.Invert(ol)
		}

		oligos = append(oligos, ol)
		return nil
	})

	return oligos, err
}

func Parse(fname string, process func(id, sequence string, quality []byte, reverse bool) error) error {
	var r io.Reader

	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()

	if cf, err := gzip.NewReader(f); err == nil {
		r = cf
	} else {
		f.Seek(0, 0)
		r = f

	}

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var id string

		l := sc.Text()
		ls := strings.Split(l, ",")
		if len(ls) == 1 {
			ls = strings.Split(l, " ")
		}

		seq := ls[0]
		if len(ls) > 1 {
			id = ls[1]
		}

		if err := process(id, seq, nil, false); err != nil {
			return err
		}
	}

	return nil
}
