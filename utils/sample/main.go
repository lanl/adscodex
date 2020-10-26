package main

import (
	"flag"
	"fmt"
	"errors"
	"math/rand"
	"os"
	"sort"
	"acoma/io/fastq"
)

var num = flag.Int("n", 0, "number of sequences to select");

func main() {

	flag.Parse()

	if *num == 0 {
		fmt.Fprintf(os.Stderr, "expected number of sequnces to output\n")
		return
	}

	count := 0
	fastq.Parse(flag.Arg(0), func(id, sequence string, quality []byte, reverse bool) error {
		count++
		return nil
	})

	if *num > count {
		fmt.Fprintf(os.Stderr, "number of input sequences smaller than the requested number to output\n")
		return
	}

	sids := make([]int, count)
	for i := 0; i < count; i++ {
		sids[i] = i
	}

	rand.Shuffle(count, func(i, j int) {
		sids[i], sids[j] = sids[j], sids[i]
	})

	seqs := sids[0:*num]
	sort.Ints(seqs)
	idx := 0
	n := 0
	fastq.Parse(flag.Arg(0), func(id, sequence string, quality []byte, reverse bool) error {
		if n == seqs[idx] {
			r := "1"
			if reverse {
				r = "2"
			}

			fmt.Printf("%s %s\n", id, r);
			fmt.Printf("%s\n", sequence);
			fmt.Printf("+\n");
			ql := "";
			for _, q := range quality {
				ql += string(q + 33)
			}
			fmt.Printf("%s\n", ql);
			idx++
			if idx == len(seqs) {
				return errors.New("EOF")
			}
		}

		n++
		return nil
	})
}
