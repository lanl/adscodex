package l1

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func ReadEntries(fname string) (ret []*Entry, err error) {
	var f *os.File
	var r io.Reader
	var n int
	var v64 uint64

	f, err = os.Open(fname)
	if err != nil {
		return
	}
	defer f.Close()

	if cf, err := gzip.NewReader(f); err == nil {
		r = cf
	} else {
		r = f
		f.Seek(0, 0)
	}

	sc := bufio.NewScanner(r)
	n = 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if line == "" {
			continue
		}

		en := new(Entry)
		ls := strings.Split(line, " ")
		if len(ls) == 1 {
			// support both space-separated and comma-separated
			ls = strings.Split(line, ",")
		}

		if len(ls) < 5 {
			err = fmt.Errorf("%d: invalid line: %s", n, line)
		}

		v64, err = strconv.ParseUint(ls[0], 10, 64)
		if err != nil {
			err = fmt.Errorf("%d: invalid address: %v: %v", n, ls[0], err)
			return
		}
		en.Addr = v64

		switch ls[1] {
		case "true":
			en.EcFlag = true

		case "false":
			en.EcFlag = false

		default:
			err = fmt.Errorf("%d: invalid EC flag: %v", n, ls[2])
			return
		}

		v64, err = strconv.ParseUint(ls[2], 10, 32)
		if err != nil {
			err = fmt.Errorf("%d: invalid distance: %v: %v", n, ls[2], err)
			return
		}
		en.Dist = int(v64)

		v64, err = strconv.ParseUint(ls[3], 10, 32)
		if err != nil {
			err = fmt.Errorf("%d: invalid count: %v: %v", n, ls[3], err)
			return
		}
		en.Count = int(v64)

		ls = ls[4:]
		en.Data = make([]byte, len(ls))
		for i := 0; i < len(ls); i++ {
			v64, err = strconv.ParseUint(ls[i], 10, 8)
			if err != nil {
				err = fmt.Errorf("%d: invalid data: %v: %v", n, ls[i], err)
				return
			}

			en.Data[i] = byte(v64)
		}

		ret = append(ret, en)
	}

	return
}

func WriteEntries(fname string, entries []*Entry) (err error) {
	var f *os.File

	if fname != "" {
		f, err = os.Create(fname)
		if err != nil {
			return
		}
		defer f.Close()
	} else {
		f = os.Stdout
	}

	for _, e := range entries {
		fmt.Fprintf(f, "%d %v %d %d", e.Addr, e.EcFlag, e.Dist, e.Count)
		for _, d := range e.Data {
			fmt.Fprintf(f, " %d", d)
		}
		fmt.Fprintf(f, "\n")
	}

	return
}
