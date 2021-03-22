package fastq

import (
        "bufio"
	"compress/gzip"
	"errors"
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
		// id line
		ls := strings.Split(sc.Text(), " ")
		if len(ls) != 2 {
			return errors.New("invalid id line: '" + sc.Text() + "'")
		}
		id := ls[0]

		// sequence
		if !sc.Scan() {
			return errors.New("expecting DNA sequence")
		}
		seq := sc.Text()

		// '+' line
		if !sc.Scan() {
			return errors.New("expecting '+' line")
		}

		// quality
		if !sc.Scan() {
			return errors.New("expecting quality line")
		}
		qual := sc.Text()
		if len(qual) != len(seq) {
			return fmt.Errorf("lengths of sequence and quality lines differ: %d:%d %v %v", len(seq), len(qual), seq, qual)
		}

		qa := make([]byte, len(qual))
		for i, c := range(qual) {
			qa[i] = byte(c) - 33 // '!'
		}

		// this is Illumina specific way to figure out if the oligo is
		// straight or reverse of a pair
		reverse := false
		if ls[1][0] == '2' {
			reverse = true
		}

		if err := process(id, seq, qa, reverse); err != nil {
			return err
		}
	}

	return nil
}
