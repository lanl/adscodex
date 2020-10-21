package l2

import (
	"crypto/sha1"
	"fmt"
	"hash/crc64"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"acoma/l0"
	"github.com/klauspost/reedsolomon"
)

type File struct {
	sync.RWMutex

	compat	bool   		// if true, use the 0.9 format (no superblocks)
	rndmz	bool		// if true, the data was randomized with the size of the file as a seed

	// initial parameters
	rows	int		// total number of rows
	cols	int		// number of columns (same as dblknum)
	elsz	int		// element size (same as blksz)
	drows	int		// number of data rows
	erows	int		// number of erasure rows
	egrpsz	int		// number of bytes per erasure group
	ec	reedsolomon.Encoder

	// erasure groups
	egrps	[]*EcGroup

	// data recovery
	complete bool		// all the data is recovered
	size	uint64		// file size
	totalsz	uint64		// total size, including the padding and the supers
	maxaddr	int64		// maximum address (totalsz / (elsz * cols))
	sha1	[]byte		// SHA1 hash of the whole file
	chunks	[]*FileChunk
	synch	chan bool	// trigger the recovery goroutine to try to recover the file, if false is sent, the goroutine exits
	closech	chan bool	// sent by the recovery goroutine before it finishes
}

type FileChunk struct {
	sha1	[]byte		// SHA1 hash for the chunk (if recovered)
	dss	[]DataExtent	// data for the chunk
}

const (
	// types for File ranges
	FileHole = 1 << iota
	FileVerified
	FileUnverified
	FileMulti
)

const (
	superSize = 8 + 20 + 8			// superblock: size, sha1, crc64
	superChunkSize = 512 * 1024		// superblock at every 512k
	maxRecoveryIterations = 65536		// maximum number of iterations to try to match the chunk SHA1
)

func newFile(egrows, egcols, elsz, ecnum int, rsenc reedsolomon.Encoder, compat bool, rndmz bool) (f *File) {
	f = new(File)
	f.compat = compat
	f.rndmz = rndmz
	f.rows = egrows
	f.cols = egcols
	f.elsz = elsz
	f.erows = ecnum
	f.drows = f.rows - f.erows
	f.egrpsz = f.drows * f.cols * f.elsz
	f.ec = rsenc

	f.synch = make(chan bool)
	f.closech = make(chan bool)
	go f.recoverproc()

	return
}

// triggers the recovery goroutine to try to recover more data
// if all data is recovered already, returns true
func (f *File) sync() bool {
	f.RLock()
	defer f.RUnlock()

	if f.complete {
		return true
	}

	// non-blocking send
	select {
	case f.synch <- true:
	}

	return false
}

func (f *File) close() (data []DataExtent) {
	f.synch <- false
	<- f.closech

	f.synch = nil
	f.closech = nil

	// ready or not, we have to prepare the data we recovered for the user
	for i, c := range f.chunks {
		if len(c.dss) != 1 {
			f.recoverData(i, c, true)
		}

		ds := c.dss
		if ds == nil {
			continue
		}

		if data != nil {
			last := &data[len(data) - 1]
			if last.Offset + uint64(len(last.Data)) == ds[0].Offset && last.Verified == ds[0].Verified {
				last.Data = append(last.Data, ds[0].Data...)
				ds = ds[1:]
			}
		}

		data = append(data, ds...)
	}

	// cut the trailing garbage at the end of the file
	if len(data) > 0 && f.size != 0 {
		for i := len(data) - 1; i >= 0; i-- {
			lds := &data[i]
			if lds.Offset > f.size {
				data = data[0:i]
				continue
			}

			if lds.Offset + uint64(len(lds.Data)) > f.size {
//				fmt.Fprintf(os.Stderr, "\t len(Data) %d trimmed %d\n", len(lds.Data), f.size - lds.Offset)
				lds.Data = lds.Data[0:f.size - lds.Offset]
			}
		}
	}

	if f.rndmz {
		if f.size == 0 {
			// we are in a big trouble, we can't unscramble the data
			return nil
		}

		rnd := rand.New(rand.NewSource(int64(f.size)))
		rndata := make([]byte, f.size)
		for i := 0; i < len(rndata); i++ {
			rndata[i] = byte(rnd.Int31n(256))
		}

		for i := 0; i < len(data); i++ {
			ds := &data[i]
			for j := 0; j < len(ds.Data); j++ {
				ds.Data[j] ^= rndata[ds.Offset + uint64(j)]
			}
		}
	}

	// TODO: check the sha1 for the whole file

	return
}

func (f *File) add(addr uint64, ef bool, dblks []Blk) bool {
	var eg *EcGroup


//	fmt.Fprintf(os.Stderr, "add %d\n", addr)
	// if we have recovered the file size, discard entries that are outside of it
	maxaddr := uint64(atomic.LoadInt64(&f.maxaddr))
	if maxaddr != 0 && addr >= maxaddr {
		return false
	}

	idx := int(addr / uint64(f.drows))
	row := int(addr % uint64(f.drows))
	if ef {
		row += f.drows
	}

	if row >= f.rows {
		return false
	}

	f.RLock()
	if idx < len(f.egrps) {
		// fast path
		eg = f.egrps[idx]
		if eg != nil {
			f.RUnlock()
			goto add
		}
	}
	f.RUnlock()

	// slow path
	f.Lock()
	if len(f.egrps) <= idx {
		// resize the array
		gs := make([]*EcGroup, idx + 1)
		copy(gs, f.egrps)
		f.egrps = gs
	}

	eg = f.egrps[idx]
	if eg == nil {
		eg = newEcGroup(f.rows, f.cols)
		f.egrps[idx] = eg
	}
	f.Unlock()

add:
	return eg.addEntry(row, dblks, f.erows, f.ec)
}

func (f *File) visit(offset uint64, count uint64, v func(addr uint64, size int, dtype int, vblks []Blk, ublks []Blk) bool) {
	f.RLock()
	defer f.RUnlock()

	end := offset + count
	idx := int(offset / uint64(f.egrpsz))	// index of the EC group
	a := int(offset % uint64(f.egrpsz))
	row := a / (f.cols * f.elsz)		// row of the element in the EC group
	a = a % (f.cols * f.elsz)
	col := a / f.elsz			// column of the element in the EC group
	rem := a % f.elsz			// offset into the element

//	fmt.Fprintf(os.Stderr, "visit: offset %d count %d idx %d row %d col %d rem %d\n", offset, count, idx, row, col, rem)
	if idx >= len(f.egrps) {
		// end of file
		return
	}

	// go over all EC groups and their elements from offset onward until
	// we get to an element that is different to what we got so far
	for ; idx < len(f.egrps); idx++ {
		eg := f.egrps[idx]
//		fmt.Fprintf(os.Stderr, "  idx %d\n", idx)
		for ; row < f.drows; row++ {
//			fmt.Fprintf(os.Stderr, "    row %d\n", row)
			for ; col < f.cols; col++ {
				var ft int
				var vd, ud []Blk

				if eg != nil {
					vd = eg.getVerified(row, col)
					ud = eg.getUnverified(row, col)
//					fmt.Fprintf(os.Stderr, "\trow %d col %d vd %v ud %v\n", row, col, vd, ud)
				}

				switch len(vd) {
				case 0:
					// nothing
				case 1:
					ft = FileVerified
				default:
					ft = FileVerified | FileMulti
				}

				if ft == 0 {
					switch len(ud) {
					case 0:
						ft = FileHole
					case 1:
						ft = FileUnverified
					default:
						ft = FileUnverified | FileMulti
					}
				}
				
				if !v(offset, f.elsz - rem, ft, vd, ud) {
					return
				}

				offset += uint64(f.elsz - rem)
				if offset >= end {
					goto out
				}

				rem = 0
			}

			col = 0
		}

		row = 0
	}

out:
	return
}

func (f *File) check(offset, count uint64) (rtype int, size uint64) {
	f.visit(offset, count, func(addr uint64, sz int, dtype int, vblks []Blk, ublks []Blk) bool {
		if sz == 0 {
			return false
		}

		if count != math.MaxUint64 && addr + uint64(sz) > offset + count {
			sz = int(offset + count - addr)
		}

		if rtype == 0 || rtype == dtype {
			rtype = dtype
			size += uint64(sz)
			return true
		} else {
			return false
		}
	})

	return
}

func (f *File) read(offset, count uint64) (rtype int, data []byte, cnt uint64) {
	f.visit(offset, count, func(addr uint64, sz int, dtype int, vblks []Blk, ublks []Blk) bool {
		if sz == 0 {
			return false
		}

		if rtype == 0 {
			rtype = dtype
		}

		if dtype&FileMulti != 0 {
			return false
		}

		if rtype != dtype {
			return false
		}

		cnt += uint64(sz)
		start := f.elsz - sz
//		fmt.Fprintf(os.Stderr, "addr %d sz %d start %d\n", addr, sz, start)
		if rtype & FileVerified != 0 {
			data = append(data, []byte(vblks[0])[start:]...)
		} else if rtype & FileUnverified != 0 {
			data = append(data, []byte(ublks[0])[start:]...)
		} else {
			// hole
		}

		return true
	})

	if uint64(len(data)) > count {
		data = data[0:count]
	}

	return
}

// Just return the data from one element
func (f *File) readMulti(offset, count uint64) (rtype int, data [][]byte, cnt uint64) {
	f.visit(offset, count, func(addr uint64, sz int, dtype int, vblks []Blk, ublks []Blk) bool {
		if sz == 0 {
			return false
		}

		rtype = dtype
		cnt = uint64(sz)
		start := f.elsz - sz
		end := f.elsz
		if count != math.MaxUint64 &&  addr + uint64(end) > offset + count {
			end = int(offset + count - addr)
		}

		blks := vblks
		if blks == nil || len(blks) == 0 {
			blks = ublks
		}

		if blks != nil && len(blks) != 0 {
			for _, b := range blks {
				d := make([]byte, sz)
				copy(d, b[start:end])
				data = append(data, d)
			}
		}

		return false
	})

	return
}

func (f *File) chunkStart(n int) uint64 {
	if f.compat {
		return uint64(n) * superChunkSize
	} else {
		return superSize + uint64(n) * (superSize + superChunkSize)
	}
}

func (f *File) readSuper(offset uint64) (size uint64, sha1 []byte) {
	if f.compat {
		return 0, nil
	}

	// Check if there are any holes
	n := 0
	fmt.Fprintf(os.Stderr, "readSuper offset %d\n", offset)
	for o := offset; o < offset + superSize; {
		t, sz := f.check(o, superSize)
//		fmt.Fprintf(os.Stderr, "readSuper: offset %d: type %x size %d\n", o, t, sz)
		if sz == 0 || t == FileHole {
			fmt.Fprintf(os.Stderr, "\tfailed\n")
			return 0, nil
		}

		o += sz
		n++
	}

	// We have all the data. It can be verified, unverified, multiple values, etc. 
	// Collect it all and try to make sense of it
	var ds [][][]byte
	for o := offset; o < offset + superSize; {
		t, sz := f.check(o, superSize)
		if t & FileMulti != 0 {
			var d [][]byte

			_, d, sz = f.readMulti(o, superSize)
			ds = append(ds, d)
		} else {
			var d []byte

			_, d, sz = f.read(o, superSize)
			ds = append(ds, [][]byte{d})
		}

		o += sz
	}

	idx := make([]int, len(ds))
	data := make([]byte, superSize)
	for done := false; !done; {
		// collect the data for the current combination
		for i, o := 0, 0; i < len(ds); i++ {
			o += copy(data[o:], ds[i][idx[i]])
		}

		// prepare the indices for the next combination
		for i := 0; i < len(idx); i++ {
			idx[i]++
			if idx[i] < len(ds[i]) {
				break
			} else {
				idx[i] = 0

				// check if we exhausted all combinations
				done = i+1 == len(idx)
			}
		}

		// now check if the combination is valid, i.e. the content matches the checksum
		ocrc, _ := l0.Gint64(data[superSize - 8:])
		ncrc := crc64.Checksum(data[0:superSize - 8], crctbl)
		if ocrc != ncrc {
			continue
		}

		// it looks that we got it
		size, _ = l0.Gint64(data)
		sha1 = data[8:superSize - 8]
		fmt.Fprintf(os.Stderr, "\tsuccess\n")
		return
	}

	// no luck
	fmt.Fprintf(os.Stderr, "\tfailed\n")
	return
}

func (f *File) updateMaxAddr() {
	n := f.size / superChunkSize
	if f.size % superChunkSize != 0 {
		n++
	}

	sz := (n + 2) * superSize + f.size
	grps := sz / uint64(f.egrpsz)
	if sz % uint64(f.egrpsz) != 0 {
		grps++
	}

	maxaddr := grps * uint64(f.drows)
	f.totalsz = maxaddr * uint64(f.elsz * f.cols)

	// we use it outside recoverproc, so it has to be atomically read and written
	atomic.StoreInt64(&f.maxaddr, int64(maxaddr))
}

func (f *File) recoverData(cnum int, c *FileChunk, force bool) (complete bool) {
	var ds []DataExtent
	var dss [][][]byte
	var idx []int
	var data []byte
	var end uint64
	var maxidx int
	var vmulti, uvmulti int		// for statistics only

	offset := f.chunkStart(cnum)
	if f.totalsz != 0 && offset > uint64(f.totalsz) {
		return true
	}

	origOff := uint64(cnum * superChunkSize)
	chunklen := uint64(superChunkSize)
	if origOff + chunklen > f.size {
		chunklen = f.size - origOff
	}

	fmt.Fprintf(os.Stderr, "recover data for chunk %d force %v offset %d %x\n", cnum, force, offset, c.sha1)
	if c.sha1 == nil || f.compat {
//		fmt.Fprintf(os.Stderr, "\tnot recovered\n")
		goto nosha1
	}

	// at this point we know there is sha1 so we do our best to match it
	// collect the data
	end = offset + chunklen
	for o := offset; o < end; {
		t, sz := f.check(o, end - o)
		if sz == 0 || t & FileHole != 0 {
			fmt.Fprintf(os.Stderr, "\thole at %d size %d\n", o, sz)
			// if there are holes revert to the same case as if sha1 is nil
			goto nosha1
		}

		if t & FileMulti != 0 {
			var d [][]byte

			_, d, sz = f.readMulti(o, end - o)
			dss = append(dss, d)
			if t & FileVerified != 0 {
				vmulti++
			} else {
				uvmulti++
			}
		} else {
			var d []byte

			_, d, sz = f.read(o, end - o)
			dss = append(dss, [][]byte{d})
		}

		o += sz
	}

	if len(dss) > 32 {	// FIXME: not sure what number to put here
		fmt.Fprintf(os.Stderr, "\t%d false positives: %d verified %d unverified\n", vmulti+uvmulti, vmulti, uvmulti)
		maxidx = 2	// FIXME: not sure what number to put here either
	}

	idx = make([]int, len(dss))
	data = make([]byte, end - offset)
	for n, done := 0, false; !done && n < maxRecoveryIterations; n++ {
		// collect the data for the current combination
		for i, o := 0, 0; i < len(dss); i++ {
			o += copy(data[o:], dss[i][idx[i]])
		}

		// prepare the indices for the next combination
		for i := 0; i < len(idx); i++ {
			idx[i]++
			if (maxidx==0 || idx[i]<maxidx) && idx[i] < len(dss[i]) {
				break
			} else {
				idx[i] = 0

				// check if we exhausted all combinations
				done = i+1 == len(idx)
			}
		}

		// now check if the combination is valid, i.e. the content matches the checksum
		ash1 := sha1.Sum(data)
		sh1 := ash1[:]
		match := true
		for i, v := range sh1 {
			if v != c.sha1[i] {
				match = false
				break
			}
		}

		if !match {
			continue
		}

		// we got it
		c.dss = []DataExtent{ DataExtent{ origOff, data, true } }
		fmt.Fprintf(os.Stderr, "\trecovered\n")
		return true
	}

nosha1:
	if !force {
		fmt.Fprintf(os.Stderr, "\tfailed\n")
		return false
	}

	// we are forced to return the data we have, no sha1 so there is no point
	// to go over combinations
	var verified bool
	var off uint64

	for o := uint64(0); o < chunklen; {
		var v bool
		var d []byte

		t, sz := f.check(offset + o, chunklen - o)
		if sz == 0 {
			break
		}

		v = t & FileVerified != 0
		if t & FileMulti != 0 {
			_, ds, _ := f.readMulti(offset + o, chunklen - o)
			d = ds[0]	// just pick the first?

			// if there were multiple values, we can't be sure we are returning the correct one
//			fmt.Fprintf(os.Stderr, "\t%d: false positive len %d count %d\n", offset + o, len(d), len(ds))
			v = false
		} else {
			_, d, _ = f.read(offset + o, chunklen - o)
		}

		// try to combine
		if (off + uint64(len(data))) != o || verified != v {
			if len(data) != 0 {
				ds = append(ds, DataExtent{ origOff + off, data, verified })
			}

			verified = v
			data = d
			off = o
		} else {
			data = append(data, d...)
		} 

		o += sz
	}

	if data != nil {
		ds = append(ds, DataExtent{ origOff + off, data, verified })	// append the last extent
	}
	c.dss = ds

	fmt.Fprintf(os.Stderr, "\trecovered %d extents\n", len(c.dss))
//	for i := 0; i < len(ds); i++ {
//		fmt.Fprintf(os.Stderr, "\t\t%d %d %v\n", ds[i].Offset, len(ds[i].Data), ds[i].Verified)
//	}
	return false
}

func (f *File) recoverproc() {
	done := false
	for !done {
		done = !<-f.synch

		fmt.Fprintf(os.Stderr, "starting recover\n")

		f.RLock()
		// calculate number of chunks based on it and resize the chunks array if needed
		fsz := uint64(len(f.egrps)) * uint64(f.egrpsz)
		chunknum := int(fsz / superChunkSize)
		if fsz % superChunkSize != 0 {
			chunknum++
		}
		f.RUnlock()

		if len(f.chunks) != chunknum {
			// this is safe without locks, because this goroutine is the only one that touches it
			c := make([]*FileChunk, chunknum)
			copy(c, f.chunks)
			for i := len(f.chunks); i < len(c); i++ {
				c[i] = new(FileChunk)
			}

			f.chunks = c
		}

		if !f.compat {
			// try to recover the file header and footer, if not recovered already
			if f.sha1 == nil {
				fmt.Fprintf(os.Stderr, "try to recover file header\n")
				sz, sha1 := f.readSuper(0)
				if sha1 == nil {
					tsz := f.totalsz
					if tsz == 0 {
						// we don't know the total size yet, try the end of the erasure groups as of now
						tsz = fsz
					}

					fmt.Fprintf(os.Stderr, "try to recover file footer\n")
					sz, sha1 = f.readSuper(tsz - superSize)
				}

				if sha1 != nil {
					fmt.Fprintf(os.Stderr, "got file information\n")
					f.sha1 = sha1
					if f.size == 0 {
						f.size = sz
						f.updateMaxAddr()
					}
				}
			}
		} else {
			// compatibility mode
			// try to recover the file size from the end of the file
			t, data, cnt := f.read(fsz - 8, 8)
			if t == FileVerified && cnt == 8 {
				sz, _ := l0.Gint64(data)

				// check if the value makes sense (i.e. within the last EC group)
				if sz+8 > (fsz - uint64(f.egrpsz)) &&  sz+8 < fsz {
					f.size = sz
				}
			}
		}

		// try to recover superblocks for each chunk that doesn't have it recovered
		fmt.Fprintf(os.Stderr, "%d chunks totalsz %d\n", len(f.chunks), f.totalsz)
		for i, c := range f.chunks {
			last := false
			if c.sha1 != nil {
				continue
			}

			offset := f.chunkStart(i) + superChunkSize
			if f.totalsz != 0 && offset > uint64(f.totalsz) {
				offset = f.chunkStart(i) + (f.size % superChunkSize)
//				offset = uint64(f.totalsz) - 2 * superSize	// footer + last chunk super
				last = true
			}

			fmt.Fprintf(os.Stderr, "try to recover information for chunk %d\n", i)
			sz, sha1 := f.readSuper(offset)
			if sha1 != nil {
				c.sha1 = sha1
				if f.size == 0 {
					f.size = sz
					f.updateMaxAddr()
				}
			}

			// if this is the last chunk (according to totalsz), get out of the loop
			if last {
				break
			}
		}

		// try to recover data from each chunk that is not complete
		cmpl := true
		for i, c := range f.chunks {
			if len(c.dss) != 1 {
				cm := f.recoverData(i, c, false)
				cmpl = cmpl && cm
			}
		}

		if cmpl {
			f.Lock()
			f.complete = true
			f.Unlock()
		}
	}

	f.closech <- true
}
