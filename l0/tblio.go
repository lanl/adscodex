package l0

// Functions for reading and writing lookup tables
// TODO: describe the on-disk format
import (
	"errors"
	"fmt"
	"io"
	"os"
	"acoma/oligo/short"
	"acoma/criteria"
)

var tblPath = "../tbl"

func SetLookupTablePath(path string) {
	tblPath = path
}

func (tbl *Table) Write(w io.Writer) (err error) {
	var buf []byte

	if tbl == nil {
		buf = Pint32(0, buf)
		buf = Pint32(0, buf)
		buf = Pint64(0, buf)
		buf = Pint64(0, buf)
		buf = Pint32(0, buf)
	} else {
		buf = Pint32(uint32(tbl.bits), buf)
		buf = Pint32(uint32(tbl.prefix.Len()), buf)
		buf = Pint64(tbl.prefix.Uint64(), buf)
		buf = Pint64(tbl.maxval, buf)
		buf = Pint32(uint32(len(tbl.tbl)), buf)

		for _, n := range tbl.tbl {
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

	buf := make([]byte, 28)

	n, err = r.Read(buf)
	if err != nil {
		return
	} else if n != 28 {
		return errors.New("short read")
	}

	v, p = Gint32(buf)
	tbl.bits = int(v)
	v, p = Gint32(p)
	v64, p = Gint64(p)
	tbl.prefix = short.Val(int(v), v64)

	tbl.maxval, p = Gint64(p)
	v, p = Gint32(p)
	tlen := int(v)

	if tlen == 0 {
		return
	}

	tbl.tbl = make([]uint64, tlen)
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
		tbl.tbl[i] = v64
	}

	return
}

func readLookupTable(fname string, crit criteria.Criteria) (lt *LookupTable, err error) {
	var f *os.File
	var v uint32
	var id uint64
	var n int

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, 16)
	n, err = f.Read(buf)
	if err != nil {
		return
	} else if n != len(buf) {
		return nil, errors.New("short read")
	}

	id, buf = Gint64(buf)
	if (id>>48) != ('L'<<8 | '0') {
		return nil, fmt.Errorf("not ACOMA lookup table: got %x expected %x", id>>48, 'L'<<8 | '0')
	}

	id &= 0xFFFFFFFFFFFF	// 48 bits
	if id != crit.Id() {
		return nil, fmt.Errorf("criteria mistmatch: got %x expected %x", id, crit.Id())
	}
	
	lt = new(LookupTable)
	v, buf = Gint32(buf)
	lt.oligolen = int(v)
	v, buf = Gint32(buf)
	lt.pfxlen = int(v)

	lt.pfxtbl = make([]*Table, (1<<(2*lt.pfxlen)))
	for i := 0; i < len(lt.pfxtbl); i++ {
		tbl := new(Table)
		err = tbl.Read(f)
		if err != nil {
			return
		}

		if tbl.tbl == nil {
			tbl = nil
		}

		lt.pfxtbl[i] = tbl
	}

	return
}

func (lt *LookupTable) Write(fname string) (err error) {
	var f *os.File

	f, err = os.Create(fname)
	if err != nil {
		return
	}
	defer f.Close()

	id := ('L'<<56 | '0'<<48) | lt.crit.Id()
	buf := Pint64(id, nil)
	buf = Pint32(uint32(lt.oligolen), buf)
	buf = Pint32(uint32(lt.pfxlen), buf)
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
