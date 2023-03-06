package main

import (
	"flag"
	"fmt"
	"adscodex/oligo"
	"adscodex/oligo/long"
//	"sort"
)

func main() {
	flag.Parse()


	o1, _ := long.FromString(flag.Arg(0))
	o2, _ := long.FromString(flag.Arg(1))

	d, s := oligo.Diff(o1, o2)
	fmt.Printf("%d %v\n", d, s)
}
