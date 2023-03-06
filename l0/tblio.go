package l0

// Functions for reading and writing lookup tables
// TODO: describe the on-disk format
import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"adscodex/oligo/short"
	"adscodex/criteria"
)

func (tbl *Table) Write(w io.Writer) (err error) {
	var buf []byte

	if tbl == nil {
		buf = Pint32(0, buf)
		buf = Pint64(0, buf)
		buf = Pint32(0, buf)
		buf = Pint32(0, buf)
		buf = Pint32(0, buf)
	} else {
		buf = Pint32(uint32(tbl.prefix.Len()), buf)
		buf = Pint64(tbl.prefix.Uint64(), buf)
		buf = Pint32(uint32(tbl.olen), buf)
		buf = Pint32(uint32(tbl.mindist), buf)
		buf = Pint32(uint32(len(tbl.etbl)), buf)

		for _, n := range tbl.etbl {
			buf = Pint64(n, buf)
		}
	}

	_, err = w.Write(buf)
	return
}

func (tbl *Table) Read(r io.Reader) (err error) {
	var n int
	var v uint32
	var v64 uint64
	var p []byte

	buf := make([]byte, 24)

	n, err = r.Read(buf)
	if err != nil {
		return
	} else if n != 24 {
		return errors.New("short read")
	}

	v, p = Gint32(buf)
	v64, p = Gint64(p)
	if v != 0 {
		tbl.prefix = short.Val(int(v), v64)
	}

	v, p = Gint32(p)
	tbl.olen = int(v)

	v, p = Gint32(p)
	tbl.mindist = int(v)

	v, p = Gint32(p)
	tlen := int(v)
	tbl.maxval = uint64(tlen)


	tbl.dmap = make(map[uint64]int)
	if tlen == 0 {
		return
	}

	tbl.etbl = make([]uint64, tlen)
	buf = make([]byte, tlen * 8)
	n, err = r.Read(buf)
	if err != nil {
		return
	} else if n != tlen*8 {
		return errors.New("short read")
	}

	p = buf
	for i := 0; i < tlen; i++ {
		v64, p = Gint64(p)
		tbl.etbl[i] = v64
	}

	tbl.trie, _ = NewTrie(nil)
	for i := 0; i < tlen; i++ {
		ol := short.Val(tbl.olen, tbl.etbl[i])
		tbl.trie.add(ol, 0)
		tbl.dmap[tbl.etbl[i]] = i
	}

//	fmt.Printf("trie size %d\n", tbl.trie.Size())
	return
}

func LoadLookupTable(fname string) (lt *LookupTable, err error) {
	var f *os.File
	var v uint32
	var id uint64
	var n int

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, 20)
	n, err = f.Read(buf)
	if err != nil {
		return
	} else if n != len(buf) {
		return nil, errors.New("short read")
	}

	id, buf = Gint64(buf)
	if (id>>48) != ('L'<<8 | 'V') {
		return nil, fmt.Errorf("not ADS Codex lookup table: got %x expected %x", id>>48, 'L'<<8 | 'V')
	}

	id &= 0xFFFFFFFFFFFF	// 48 bits
	crit := criteria.FindById(id)
	if crit == nil {
		return nil, fmt.Errorf("criteria with id %x not found", id)
	}

	lt = new(LookupTable)
	lt.crit = crit

	v, buf = Gint32(buf)
	lt.oligolen = int(v)
	v, buf = Gint32(buf)
	lt.pfxlen = int(v)
	v, buf = Gint32(buf)
	lt.mindist = int(v)

//	fmt.Printf("Lookup Table: criteria %v olen %d pfxlen %d mindist %d\n", crit, lt.oligolen, lt.pfxlen, lt.mindist)
	lt.pfxtbl = make([]*Table, (1<<(2*lt.pfxlen)))
	lt.maxval = math.MaxUint64
	for i := 0; i < len(lt.pfxtbl); i++ {
//		fmt.Printf("Reading %v\n", short.Val(lt.pfxlen, uint64(i)))
		tbl := new(Table)
		err = tbl.Read(f)
		if err != nil {
			return
		}

		if tbl.etbl == nil {
			tbl = nil
		}

		lt.pfxtbl[i] = tbl
		if tbl != nil && lt.maxval > tbl.maxval {
			lt.maxval = tbl.maxval
		}
	}

	lt.register()

	return
}

func (lt *LookupTable) Write(fname string) (err error) {
	var f *os.File

	f, err = os.Create(fname)
	if err != nil {
		return
	}
	defer f.Close()

	id := ('L'<<56 | 'V'<<48) | lt.crit.Id()
	buf := Pint64(id, nil)
	buf = Pint32(uint32(lt.oligolen), buf)
	buf = Pint32(uint32(lt.pfxlen), buf)
	buf = Pint32(uint32(lt.mindist), buf)
	_, err = f.Write(buf)
	if err != nil {
		return
	}

	for _, tbl := range lt.pfxtbl {
		err = tbl.Write(f)
		if err != nil {
			return
		}
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
