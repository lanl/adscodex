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
_	"adscodex/criteria"
)

var tblPath = "../tbl"

func SetLookupTablePath(path string) {
	tblPath = path
}

func (tbl *EncTable) Write(w io.Writer, maxval int) (err error) {
	var buf []byte

	for i := 0; i < maxval; i++ {
		ol := &tbl.oligos[i]
		buf = Pint32(uint32(ol.Uint64()), buf)
	}

//	fmt.Printf("write etbl %d\n", len(buf))
	_, err = w.Write(buf)
	return
}

func (tbl *EncTable) Read(r io.Reader, olen, maxval int) (err error) {
	var n int
	var v uint32

	buf := make([]byte, maxval * 4)
	ols := make([]short.Oligo, maxval)
	tbl.oligos = ols

//	fmt.Printf("read etbl %d\n", len(buf))
	n, err = r.Read(buf)
	if err != nil {
		return
	} else if n != len(buf) {
		return errors.New("short read")
	}

	p := buf
	for i := 0; i < len(ols); i++ {
		v, p = Gint32(p)
		ols[i].SetVal(olen, uint64(v))
	}

	if len(p) != 0 {
		panic("internal error")
	}

	return
}

func (tbl *DecTable) Write(w io.Writer, olen, vrntnum int) (err error) {
	var buf []byte

	for i := 0; i < 1 << (2 * olen); i++ {
		for j := 0; j < vrntnum; j++ {
			v := &tbl.entries[i][j]
			buf = Pint16(v.val, buf)
			buf = Pint8(byte(v.ol.Len()), buf)
			buf = Pint32(uint32(v.ol.Uint64()), buf)
			buf = Pint32(math.Float32bits(v.prob), buf)
		}
	}

//	fmt.Printf("write dtbl %d (%d)\n", len(buf), olen)
	_, err = w.Write(buf)
	return
}

func (tbl *DecTable) Read(r io.Reader, olen, vnum int) (err error) {
	var n int
	var v8 byte
	var v16 uint16
	var v32 uint32

	onum := 1 << (2*olen)
	buf := make([]byte, onum*vnum*(2+1+4+4))
	ents := make([][VariantNum]DecVariant, onum)
	tbl.entries = ents

//	fmt.Printf("read dtbl %d\n", len(buf))
	n, err = r.Read(buf)
	if err != nil {
		return
	} else if n != len(buf) {
		return fmt.Errorf("short read %d expected %d", n, len(buf))
	}

	p := buf
	for i := 0; i < onum; i++ {
		for j := 0; j < vnum; j++ {
			v := &ents[i][j]

			v16, p = Gint16(p)
			v.val = v16
			v8, p = Gint8(p)
			v32, p = Gint32(p)
			v.ol.SetVal(int(v8), uint64(v16))
			v32, p = Gint32(p)
			v.prob = math.Float32frombits(v32)
		}
	}

	if len(p) > 0 {
		panic("internal error")
	}

	return
}

func readLookupTable(fname string) (lt *LookupTable, err error) {
	var f *os.File
	var v uint32
	var n int

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	buf := make([]byte, 4+4+4+4)
	n, err = f.Read(buf)
	if err != nil {
		return
	} else if n != len(buf) {
		return nil, errors.New("short read")
	}

	// header
	lt = new(LookupTable)
	v, buf = Gint32(buf)
	lt.oligoLen = int(v)
	v, buf = Gint32(buf)
	lt.maxVal = int(v)
	v, buf = Gint32(buf)
	lt.pfxLen = int(v)
	v, buf = Gint32(buf)
	lt.vrntNum = int(v)

	if lt.vrntNum > VariantNum {
		lt.vrntNum = VariantNum
	}

//	fmt.Printf("olen %d maxval %d pfxlen %d variantnum %d\n", lt.oligoLen, lt.maxVal, lt.pfxLen, lt.vrntNum)
	lt.etbls = make([]*EncTable, (1<<(2*lt.pfxLen)))
	lt.dtbls = make([]*DecTable, (1<<(2*lt.pfxLen)))

	// encoding tables
	for i := 0; i < len(lt.etbls); i++ {
//		fmt.Printf("etable %d\n", i)
		tbl := new(EncTable)
		err = tbl.Read(f, lt.oligoLen, lt.maxVal)
		if err != nil {
			return
		}

		if tbl.oligos == nil {
			tbl = nil
		}

		lt.etbls[i] = tbl
	}

	// decoding tables
	for i := 0; i < len(lt.dtbls); i++ {
//		fmt.Printf("dtable %d\n", i)
		tbl := new(DecTable)
		err = tbl.Read(f, lt.oligoLen, lt.vrntNum)
		if err != nil {
			return
		}

		lt.dtbls[i] = tbl
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

	// header
	buf := Pint32(uint32(lt.oligoLen), nil)
	buf = Pint32(uint32(lt.maxVal), buf)
	buf = Pint32(uint32(lt.pfxLen), buf)
	buf = Pint32(uint32(lt.vrntNum), buf)
	_, err = f.Write(buf)
	if err != nil {
		return
	}

	// encoding tables
//	fmt.Printf("encode table %d decode table %d\n", len(lt.etbls), len(lt.dtbls))
	for _, tbl := range lt.etbls {
		err = tbl.Write(f, lt.maxVal)
		if err != nil {
			return
		}
	}

	// decoding tables
	for _, tbl := range lt.dtbls {
		err = tbl.Write(f, lt.oligoLen, lt.vrntNum)
		if err != nil {
			return
		}
	}

	return
}

func Gint8(buf []byte) (byte, []byte) {
	return buf[0], buf[1:]
}

func Gint16(buf []byte) (uint16, []byte) {
	return uint16(buf[0]) | (uint16(buf[1]) << 8), buf[2:]
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

func Pint8(val byte, buf []byte) []byte {
	buf = append(buf, val)
	return buf
}

func Pint16(val uint16, buf []byte) []byte {
	buf = append(buf, uint8(val))
	buf = append(buf, uint8(val >> 8))
	return buf
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
