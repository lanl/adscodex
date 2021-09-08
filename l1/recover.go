package l1

import (
	"fmt"
	"adscodex/oligo"
_	"adscodex/oligo/long"
_	"adscodex/l0"
)

func (c *Codec) tryDecode(p5, p3, ol oligo.Oligo, difficulty int) (addr uint64, ef bool, data []byte, err error) {
	var prefix oligo.Oligo

	vnum := difficulty + 1
	ol, prefix = c.cutPrimers(p5, p3, ol)
	if ol == nil {
		err = Eprimer
		return
	}

	blks := make([]byte, c.datanum + c.mdnum + c.cmdnum)

	// recursively try to decode each byte
	ok := c.extractBytes(prefix, ol, 0, blks, vnum)
	if !ok {
		err = fmt.Errorf("can't decode")
		return
	}

	addr, ef, data, err = c.extractOligo(blks)
	return
}

func (c *Codec) extractBytes(prefix, ol oligo.Oligo, idx int, blks []byte, vnum int) (ok bool) {
	plen := c.c0.PrefixLen()
	if idx == len(blks) {
//		if ol.Len() == 0 {
//			fmt.Printf("\n")
//		} else {
//			fmt.Printf("X\n")
//		}
		return ol.Len() == 0
	}

	if ol.Len() < c.c0.OligoLen() {
		// short oligo
//		fmt.Printf("X\n")
		return
	}

	o := ol.Slice(0, c.c0.OligoLen())
	vs, err := c.c0.Decode(prefix, o)
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(vs) && i < vnum; i++ {
//		for n := 0; n < idx; n++ {
//			fmt.Printf(" ")
//		}
//
//		fmt.Printf("%d (%v|%v|%v)\n", vs[i].Val, prefix, o, &vs[i].Ol)

		blks[idx] = byte(vs[i].Val)
		p := prefix.Clone()
		p.Append(&vs[i].Ol)
		ok = c.extractBytes(p.Slice(p.Len() - plen, 0), ol.Slice(vs[i].Ol.Len(), 0), idx + 1, blks, vnum)
		if ok {
			break
		}
	}

//	fmt.Printf("\n")
	return
}

func (c *Codec) cutPrimers(p5, p3, ol oligo.Oligo) (olcut, prefix oligo.Oligo) {
	// TODO: fix this
	plen := c.c0.PrefixLen()
	if p5.Len() < plen || p3.Len() < plen {
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
	prefix = p5.Slice(p5.Len() - plen, p5.Len())

	return
}

func (c *Codec) extractOligo(blks []byte) (addr uint64, ef bool, data []byte, err error) {
	var sf, ok bool

	data = make([]byte, c.datanum)
	md := make([]uint64, c.mdnum + c.cmdnum)

	didx := 0
	mdidx := 0
	for i := 0; i < len(blks); {
		data[didx] = blks[i]
		didx++
		i++

		if mdidx < len(md) {
			md[mdidx] = uint64(blks[i])
			mdidx++
			i++
		}
	}

	// check if the checksum is correct
	ok, err = c.checkMDBlocks(md)
	if !ok {
		return
	}

	for i := 0; i < c.mdnum; i++ {
		addr |= uint64(md[i]) << (8 * i)
	}

//	fmt.Printf("mdblocks %v %v\n", md, addr)
	maxaddr := c.MaxAddr()
	if addr >= 2*maxaddr {
		sf = true
		addr -= 2*maxaddr
	}

	if addr >= maxaddr {
		ef = true
		addr -= maxaddr
	}

	if sf {
		// invert the data
		for i := 0; i < len(data); i++ {
			data[i] = ^data[i]
		}
	}

	return
}
