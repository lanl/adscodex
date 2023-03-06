package l0

// Functions for reading and writing lookup tables
// TODO: describe the on-disk format
import (
	"errors"
	"fmt"
_	"math"
	"math/rand"
	"os"
	"sync"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/short"
_	"adscodex/oligo/long"
_	"adscodex/criteria"
)

type Codec struct {
	sync.Mutex
	olen	int
	maxtime	int64
	etbl	[]uint64
	dmap	map[uint64]int
	trie	*Trie
	rnd	*rand.Rand
}

// maxtime is in ms
func New(tblName string, maxtime int64) (c *Codec, err error) {
	c = new(Codec)
	c.maxtime = maxtime
	c.dmap = make(map[uint64]int)

	c.olen, c.etbl, err = ReadTable(tblName)
	if err != nil {
		c = nil
		return
	}

	c.trie = new(Trie)
	for n, o := range c.etbl {
		c.dmap[o] = n
		ol := short.Val(c.olen, o)
		c.trie.add(ol, 0)
	}

	c.rnd = rand.New(rand.NewSource(time.Now().UnixMilli()))
	return
}

func (c *Codec) match(ol oligo.Oligo) (ret oligo.Oligo, dist int) {

	// create random order of bps per position
	bporder := make([]int, c.olen * 4)
	for i := 0; i < len(bporder); i++ {
		bporder[i] = i%4
	}

	c.Lock()
	for i := 0; i < c.olen; i++ {
		n := i*4
		c.rnd.Shuffle(4, func(i, j int) {
			bporder[n+i], bporder[n+j] = bporder[n+j], bporder[n+i]
		})
	}
	c.Unlock()

	stoptime := int64(-1)
	if c.maxtime > 0 {
		stoptime = time.Now().Add(time.Duration(c.maxtime) * time.Millisecond).UnixMilli()
	}

	m := c.trie.SearchMin(ol, bporder, stoptime)
	if m != nil {
		ret = m.Seq
		dist = m.Dist
	}

	return
}

func (c *Codec) Encode(val uint64) (ret oligo.Oligo, err error) {
	if val >= uint64(len(c.etbl)) {
		err = fmt.Errorf("value too big")
		return
	}

	ret = short.Val(c.olen, c.etbl[val])
	return
}

func (c *Codec) Decode(ol oligo.Oligo) (val uint64, dist int, err error) {
	var v int

	if ol == nil {
		panic("ol is nil")
	}

	match, d := c.match(ol)
	if match == nil {
		err = fmt.Errorf("no match")
		return
	}

	if match.Len() != c.olen {
		panic("shouldn't happen")
	}

//	fmt.Printf("match %v\n", match)
	dist = d
	sol, ok := short.Copy(match)
	if !ok {
		panic("shouldn't happen")
	}

	v, ok = c.dmap[sol.Uint64()]
	if !ok {
		panic("shouldn't happen")
	}

	val = uint64(v)
	return
}

func (c *Codec) MaxVal() uint64 {
	return uint64(len(c.etbl))
}

func (c *Codec) OligoLen() int {
	return c.olen
}

func WriteTable(olen int, tbl []uint64, fname string) (err error) {
	var f *os.File
	var buf []byte

	f, err = os.Create(fname)
	if err != nil {
		return
	}
	defer f.Close()

	if tbl == nil {
		buf = Pint32(0, buf)
		buf = Pint64(0, buf)
	} else {
		buf = Pint32(uint32(olen), buf)
		buf = Pint64(uint64(len(tbl)), buf)
		for _, n := range tbl {
			buf = Pint64(n, buf)
		}
	}

	_, err = f.Write(buf)
	return
}


func ReadTable(fname string) (olen int, tbl []uint64, err error) {
	var f *os.File
	var n int
	var v uint32
	var v64 uint64
	var p []byte

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, 12)
	n, err = f.Read(buf)
	if err != nil {
		return
	} else if n != 12 {
		err = errors.New("short read")
		return
	}

	v, p = Gint32(buf)
	v64, p = Gint64(p)
	if v != 0 {
		olen = int(v)
	}

	tbl = make([]uint64, v64)
	buf = make([]byte, v64*8)
	n, err = f.Read(buf)
	if err != nil {
		return
	} else if uint64(n) != v64*8 {
		err = errors.New("short read")
		return
	}

	p = buf
	for i := 0; i < len(tbl); i++ {
		v64, p = Gint64(p)
		tbl[i] = v64
	}

	return
}

func Gint32(buf []byte) (uint32, []byte) {
	return uint32(buf[0]) | (uint32(buf[1]) << 8) | (uint32(buf[2]) << 16) |
			(uint32(buf[3]) << 24),
		buf[4:]
}

func Gint64(buf []byte) (uint64, []byte) {
	return uint64(buf[0]) | (uint64(buf[1]) << 8) | (uint64(buf[2]) << 16) |
			(uint64(buf[3]) << 24) | (uint64(buf[4]) << 32) | (uint64(buf[5]) << 40) |
			(uint64(buf[6]) << 48) | (uint64(buf[7]) << 56),
		buf[8:]
}

func Pint32(val uint32, buf []byte) []byte {
	buf = append(buf, uint8(val))
	buf = append(buf, uint8(val >> 8))
	buf = append(buf, uint8(val >> 16))
	buf = append(buf, uint8(val >> 24))
	return buf
}

func Pint64(val uint64, buf []byte) []byte {
	buf = append(buf, uint8(val))
	buf = append(buf, uint8(val >> 8))
	buf = append(buf, uint8(val >> 16))
	buf = append(buf, uint8(val >> 24))
	buf = append(buf, uint8(val >> 32))
	buf = append(buf, uint8(val >> 40))
	buf = append(buf, uint8(val >> 48))
	buf = append(buf, uint8(val >> 56))
	return buf
}
