package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"adscodex/criteria"
	"adscodex/oligo/long"
)

var crit = flag.String("c", "h4g2", "criteria")

func main() {

	flag.Parse()
	c := criteria.Find(*crit)
	if c == nil {
		fmt.Fprintf(os.Stderr, "Error: invalid criteria\n")
		return
	}

	sc := bufio.NewScanner(os.Stdin)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if line == "" {
			continue
		}

		ls := strings.Split(line, " ")
		ol, ok := long.FromString(ls[0])
		if !ok {
			fmt.Fprintf(os.Stderr, "%d: invalid sequence: %s", n, ls[9])
			return
		}

		if c.Check(ol) {
			fmt.Printf("%v\n", line)
		}
	}


}
