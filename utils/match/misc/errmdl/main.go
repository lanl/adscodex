// Print number of reads (or cubundance) per original oligo.
// Uses match file from the match utility
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/utils/match/file"
)

type Op int
const (
	Root	Op = iota
	Ins
	Del
	Sub
)

type Node struct {
	sync.Mutex
	op	Op		// operation
	nt	int		// nucleotide
	nt2	int		// for substitution
	count	uint64

	parent	*Node
	children []*Node
}

var opmap = []string {
	Root: "Root",
	Ins: "I",
	Del: "D",
	Sub: "R",
}

var errnum = flag.Int("err", 0, "Maximum number of errors for match (0 - any)")
var p5primer = flag.String("p5", "", "5'-end primer")
var p3primer = flag.String("p3", "", "3'-end primer")
var dist = flag.Int("dist", 2, "number of errors allowed in the primers when matching")
var xprimers = flag.Bool("xp", false, "exclude primers from stats");
var showprob = flag.Bool("prob", false, "show the probability of an error sequence")

func (nd *Node) Add(op Op, nt, nt2 int) *Node {
	nd.Lock()
	defer nd.Unlock()

	for _, c := range nd.children {
		if c.op == op && c.nt == nt && c.nt2 == nt2 {
			return c
		}
	}

	c := new(Node)
	c.op = op
	c.nt = nt
	c.nt2 = nt2
	c.parent = nd
	nd.children = append(nd.children, c)
	return c
}

func (nd *Node) Visit(visit func(c *Node)) {
	visit(nd);
	for _, c := range nd.children {
		c.Visit(visit)
	}
}

func (nd *Node) String() (ret string) {
	for nd.op != Root && nd != nil {
		if nd.op == Sub {
			ret += fmt.Sprintf(" %s:%s:%s", opmap[nd.op], oligo.Nt2String(nd.nt), oligo.Nt2String(nd.nt2))
		} else {
			ret += fmt.Sprintf(" %s:%s", opmap[nd.op], oligo.Nt2String(nd.nt))
		}

		nd = nd.parent
	}

	return ret
}

func main() {
	var ok bool
	var p3, p5 oligo.Oligo
	var p3len, p5len int
	var root *Node

	var mutex sync.Mutex
	var readnum uint64
	var ntnum uint64

	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "expecting match file name\n")
		return
	}

	if *p5primer != "" {
		p5, ok = long.FromString(*p5primer)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid 5'-end primer: %v\n", *p5primer)
		}
		p5len = p5.Len()
	}

	if *p3primer != "" {
		p3, ok = long.FromString(*p3primer)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: invalid 5'-end primer: %v\n", *p5primer)
		}
		p3len = p3.Len()
	}


	root = new(Node)
	uniqnum := 0
	var err error
	for fn := 0; fn < flag.NArg(); fn++ {
		err = file.ParseParallel(flag.Arg(fn), 0, func(id, count int, diff string, cubu float64, orig, read oligo.Oligo) {
			if p5 != nil && p3 != nil {
				// ignore reads that don't have the primers
				ppos, plen := oligo.Find(read, p5, *dist)
				if ppos == -1 {
					return
				}

				spos, slen := oligo.Find(read, p3, *dist)
				if spos == -1 {
					return
				}

				if (*xprimers) {
					ppos += plen
					orig = orig.Slice(p5len, orig.Len() - p3len)
				} else {
					spos += slen
				}

				read = read.Slice(ppos, spos)
				_, diff = oligo.Diff(orig, read)
//				fmt.Printf("%v\n%v\n%v\n", orig, read, diff)
			}

			if *errnum != 0 {
				nerr := 0
				for i := 0; i < len(diff); i++ {
					if diff[i] != '-' {
						nerr++
					}
				}

				if nerr > *errnum {
					return
				}
			}

			node := root

			i, j := 0, 0
			olen := orig.Len()
			rlen := read.Len()
			depth := 0
			for a := 0; a < len(diff); a++ {
				if olen <= i || rlen <= j {
//					fmt.Printf("\nOrig: %v %d\n", orig, i)
//					fmt.Printf("Read: %v %d\n", read, j)
//					fmt.Printf("Diff: %v %d\n", diff, a)
				}
				switch diff[a] {
				case '-':
					i++
					j++

				case 'I':
					c := read.At(j)
					node = node.Add(Ins, c, 0)
					depth++
					j++

				case 'D':
					oc := orig.At(i)
					node = node.Add(Del, oc, 0)
					depth++
					i++

				case 'R':
					oc := orig.At(i)
					c := read.At(j)
					node = node.Add(Sub, c, oc)
					depth++
					i++
					j++
				}
			}

			node.Lock()
			node.count += uint64(count)
			node.Unlock()

			mutex.Lock()
			readnum += uint64(count)
			ntnum += uint64(count * olen)
			uniqnum++
			mutex.Unlock()

			if uniqnum%100000 == 0 {
				fmt.Fprintf(os.Stderr, ".")
			}
		})

		if err != nil {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "\n")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	var nodes []*Node
	root.Visit(func(nd *Node) {
		if nd.count != 0 {
			nodes = append(nodes, nd)
		}
	})

	sort.Slice(nodes, func (i, j int) bool {
		return nodes[i].count > nodes[j].count
	})

	for _, nd := range nodes {
		if *showprob {
			fmt.Printf("%v%v\n", float64(nd.count)/float64(readnum), nd)
		} else {
			fmt.Printf("%v%v\n", nd.count, nd)
		}
	}
}
