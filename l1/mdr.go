package l1

import (
        "bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
	"strconv"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
)

type Eentry struct {
	prob	float64		// probability for the error to occur
	lendiff	int		// how does the entry change the length of the oligo
	ops	[]Eop		// actions
}

type Eop struct {
	op	byte		// D, I, or R
	nt	int		// the nt that was deleted, inserted, or replaced
	nt2	int		// for R, what the nt was replaced to
}

func readErrorEntries(fname string, maxerrs int) (ents []Eentry, err error) {
	var r io.Reader

	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if cf, err := gzip.NewReader(f); err == nil {
		r = cf
	} else {
		r = f
		f.Seek(0, 0)
	}

	sc := bufio.NewScanner(r)
	var total float64
	for sc.Scan() {
		var prob float64

		line := sc.Text()
		vs := strings.Split(line, " ")
		prob, err = strconv.ParseFloat(vs[0], 64)
		if err != nil {
			return
		}

		if len(vs) - 1 > maxerrs {
			continue
		}

		total += prob
		if len(vs) == 1 {
			ents = append(ents, Eentry { prob, 0, nil })
			continue
		}

		ops := make([]Eop, len(vs) - 1)
		lendiff := 0
		for i := 0; i < len(ops); i++ {
			s := vs[i+1]
			op := &ops[i]
			ss := strings.Split(s, ":")
			if len(ss) != 2 && len(ss) != 3 {
				err = fmt.Errorf("Invalid op: %s", s)
				return
			}

			op.nt = oligo.String2Nt(ss[1])
			if op.nt == -1 {
				err = fmt.Errorf("Invalid nt: %s", ss[1])
				return
			}

			if len(ss) == 3 {
				op.nt2 = oligo.String2Nt(ss[2])
				if op.nt2 == -1 {
					err = fmt.Errorf("Invalid nt: %s", ss[2])
					return
				}
			}

			switch ss[0] {
			default:
				err = fmt.Errorf("Invalid op: %s", ss[0])
				return

			case "I":
				op.op = 'I'
				lendiff--

			case "D":
				op.op = 'D'
				lendiff++

			case "R":
				op.op = 'R'
				if len(ss) != 3 {
					err = fmt.Errorf("Substitution expects two nts: %s", s)
					return
				}
			}
		}

		ents = append(ents, Eentry { prob, lendiff, ops })
	}

	// go through the entries a second time and update the probability
	for i := 0; i < len(ents); i++ {
		ents[i].prob /= total
	}

	return
}

func (c *Codec) tryDecode(p5, p3, ol oligo.Oligo, difficulty int) (addr uint64, ef bool, data [][]byte, err error) {
	var prefix oligo.Oligo

	var prob float64
	switch difficulty {
	case 0:
		// just make it greater than 0, the "no changes" entry will take care of stopping after it
		prob = 0.00001

	case 1:
		prob = 0.95

	case 2:
		prob = 0.97

	case 3:
		prob = 0.99
	}

	ol, prefix = c.cutPrimers(p5, p3, ol)
	if ol == nil {
		err = Eprimer
		return
	}

//	fmt.Printf("tryDecode %v\n", ol)
	mdblks := make([]uint64, c.blknum)
	olen := ol.Len()
	seq := long.New(c.olen)
	for i := 0; i < len(c.ents) && prob > 0; i++ {
		e := &c.ents[i]
		prob -= e.prob

		// skip entries that are not going to produce an oligo of the expected length
//		fmt.Printf("\tolen %d lendiff %d c.olen %d ent {%v}\n", olen, e.lendiff, c.olen, &c.ents[i])
		if olen+e.lendiff != c.olen {
			continue
		}

		success := c.fixErrors(ol, seq, e, func(ol oligo.Oligo) bool {
//			fmt.Printf("  ?Decode %v ", ol)

			if !c.extractMetadata(prefix, ol, mdblks) {
//				fmt.Printf("bad md\n")
				return false
			}

			data = c.extractData(prefix, ol)

			// ensure that we extracted at least one data block
			dataok := false
			for _, dblk := range data {
				if dblk != nil {
					dataok = true
					break
				}
			}

//			fmt.Printf("%v\n", dataok)
			return dataok
		})

		if success {
			addr, ef = c.extractOligo(mdblks, data)
			return
		}
	}

	if err == nil {
		err = fmt.Errorf("can't decode")
	}

	return 0, false, nil, err
}

func (c *Codec) fixErrors(ol, olbuf oligo.Oligo, eent *Eentry, check func(o oligo.Oligo) bool) bool {
	// if it is no-op entry, check without any errors
	if len(eent.ops) == 0 {
		return check(ol)
	}

	// apply the first op to each of the position (and recursively apply the rest)
	olen := olbuf.Len()
	if ol.Len() < olen {
		olen = ol.Len()
	}

	for pos := 0; pos < olen; pos++ {
		if c.applyEops(ol, olbuf, pos, pos, eent.ops[0:], check) {
			return true
		}

		// if there is no error at this position, copy the nt from the original oligo
		olbuf.Set(pos, ol.At(pos))
	}

	return false
}

func (c *Codec) applyEops(osrc, odest oligo.Oligo, sidx, didx int, eops []Eop, check func(o oligo.Oligo) bool) bool {
	if len(eops) == 0 {
		// we reached the end of the ops, copy the rest of the oligo and do the check
		n := osrc.Len() - sidx
		if n != odest.Len() - didx {
			panic("the ends don't match")
		}

//		fmt.Printf("  |       %v\n", odest)
		for i := 0; i < n; i++ {
			odest.Set(didx + i, osrc.At(sidx + i))
		}
//		fmt.Printf("  !       %v\n", odest)

		return check(odest)
	}

	if sidx >= osrc.Len() || didx >= odest.Len() {
		return false
	}

	op := &eops[0]
	switch op.op {
	case 'D':
		odest.Set(didx, op.nt)
		didx++

	case 'I':
		if osrc.At(sidx) != op.nt {
			// the current nt doesn't match what we expected to be inserted, so we can't apply the op here
			return false
		}
		sidx++

	case 'R':
		if osrc.At(sidx) != op.nt2 {
			// the current nt doesn't match what we expected to be replaced with, so we can't apply the op here
			return false
		}

//		fmt.Printf("\t\tsidx %d didx %d\n", sidx, didx)
		odest.Set(didx, op.nt)
		sidx++
		didx++
	}

	return c.applyEops(osrc, odest, sidx, didx, eops[1:], check)
}

func (c *Codec) cutPrimers(p5, p3, ol oligo.Oligo) (olcut, prefix oligo.Oligo) {
	// TODO: fix this
	if p5.Len() < 4 || p3.Len() < 4 {
		panic("primers too short")
	}

	// First cut the primers
	pos5, len5 := oligo.Find(ol, p5, PrimerErrors)
	if pos5 != 0 {
		return
	}

	pos3, _/*len3*/ := oligo.Find(ol, p3, PrimerErrors)
	if pos3 < 0 /*|| pos3+len3 != ol.Len()*/ {
		return
	}

	olcut = ol.Slice(pos5+len5, pos3)
	prefix = p5.Slice(p5.Len() - 4, p5.Len())

	return
}

func (c *Codec) extractMetadata(prefix, ol oligo.Oligo, mdblks []uint64) bool {
	// Next, try to decode the metadata.
	mdok := true

	// collect metadata
	for i, mdpos := 0, dblkSize; i < c.blknum; i++ {
		mdsz := c.mdsz
		if i >= c.blknum - c.rsnum {
			mdsz = c.mdcsumLen()	// erasure blocks might be different size
		}

		mdpfx := ol.Slice(mdpos - 4, mdpos)
		mdol := ol.Slice(mdpos, mdpos + mdsz)
		if mdol.Len() != mdsz {
			// short oligo
			mdok = false
			break
		}

		var e error
		mdblks[i], e = l0.Decode(mdpfx, mdol, c.crit)
		if e != nil {
			mdok = false
			break
		}

		if mdblks[i] >= uint64(maxvals[c.mdsz]) {
			mdok = false
			break
		}

		mdpos += dblkSize + mdsz
	}

	// check if the erasure codes match
	if mdok {
		mdok, _ = c.checkMDBlocks(mdblks)
	}

	return mdok
}

func (c *Codec) extractData(prefix, ol oligo.Oligo) (dblks [][]byte) {
	// Next decode the data
	dpfx := prefix
	for i, dpos := 0, 0; i < c.blknum; i++ {
		var v uint64
		var d []byte
		var e error

		dol := ol.Slice(dpos, dpos + dblkSize)
		if dol.Len() != dblkSize {
			goto savedblk
		}

		v, e = l0.Decode(dpfx, dol, c.crit)
		if e == nil {
			var ok bool

			ok, d = c.checkDataBlock(v)
			if !ok {
				// just in case
				d = nil
			}
		}

savedblk:
		dblks = append(dblks, d)

		mdsz := c.mdsz
		if i >= c.blknum - c.rsnum {
			mdsz = c.mdcsumLen()
		}

		dpos += dblkSize + mdsz
		dpfx = ol.Slice(dpos - 4, dpos)
	}

	return	
}

func (c *Codec) extractOligo(mdblks []uint64, dblks [][]byte) (addr uint64, ef bool) {
	var sf bool

	// FIXME: md can be more than 64 bits
	maxval := uint64(maxvals[c.mdsz])
	md := uint64(0)
	for i := 0; i < c.blknum - c.rsnum; i++ {
		nmd := md * maxval + mdblks[i]
		if nmd < md {
			// overflow, panic!
			panic("metadata overflow")
		}
		md = nmd
	}

	maxaddr := c.MaxAddr()
	if md >= 2*maxaddr {
		sf = true
		md -= 2*maxaddr
	}

	if md >= maxaddr {
		ef = true
		md -= maxaddr
	}

	addr = md
	if sf {
		// invert the data
		for _, dblk := range dblks {
			for i := 0; i < len(dblk); i++ {
				dblk[i] = ^dblk[i]
			}
		}
	}

	return	
}

func (e *Eentry) String() (ret string) {
	ret = fmt.Sprintf("%v [", e.prob)
	for i := 0; i < len(e.ops); i++ {
		ret += fmt.Sprintf("%v ", e.ops[i])
	}

	ret += "]"
	return
}

func (eop Eop) String() (ret string) {
	ret = fmt.Sprintf("%c:%s", eop.op, oligo.Nt2String(eop.nt))
	if eop.op == 'R' {
		ret += fmt.Sprintf(":%s", oligo.Nt2String(eop.nt2))
	}

	return
}
