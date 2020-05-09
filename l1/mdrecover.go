package l1

import (
	"math/bits"
_	"fmt"
	"acoma/oligo"
	"acoma/oligo/long"
	"acoma/l0"
)

// At the moment the metadata recovery code assumes that there are no errors in the
// metadata blocks, only in the data blocks
// The code handling errors in the metadata blocks is written, but it is too slow right now
// so it is disabled. It will eventually get enabled (and tested better) if necessary


const (
	Nerrdata = 2	// up to 2 errors in a data block
	Nerrmd = 1	// up to 1 error in a metadata block. Not really used at the moment, 1 is hardcoded in the code itself (see tryMd)
)

var singleNts = []oligo.Oligo {
	long.FromString1("A"),
	long.FromString1("T"),
	long.FromString1("C"),
	long.FromString1("G"),
}

func (c *Codec) tryRecover(prefix, ol oligo.Oligo, difficulty int) (data [][]byte, mdblks []uint64, err error) {
	mdblks = make([]uint64, c.blknum)
	data = make([][]byte, c.blknum)
	n := c.tryMatch(0, prefix, ol, c.OligoLen(), mdblks, data, difficulty)	// n is the number of errors to match, <0 if no match
	if n < 0 {
		// couldn't match
		err = Emetadata
		return
	}

	return
}

// Try to shift oligo around so we can match the metadata
// Since the function is called recursively, the idx parameter indicates
// which block we are matching. The olen parameter indicates what ol's
// length is supposed to be if there are no errors.
// If match is found, the function returns the number of errors that were
// made in order to match the metadata. mdblks and data contain the 
// match.
// This function gets the oligo with the data block at the beginning. It goes over
// all possible error combinations that we allow in the data block. It strips the 
// data block according to the errors and calls tryMd to try errors in the metadata
// block. Eventually tryMd will call tryMatch with idx+1 to work on the next data block
func (c *Codec) tryMatch(idx int, prefix, ol oligo.Oligo, olen int, mdblks []uint64, data [][]byte, difficulty int) (err int) {
	err = -1
	data[idx] = nil

	// Do a common sense check before we even try to match:
	// we can have up to (blknum-idx)*(Nerrdata + 1) insertions or deletions for
	// the rest of the oligo. If the difference between olen and the actual
	// oligo length is bigger, there is no point of going that route, we'll fail anyway
	// This is very conservative and will probably easily pass in the first recursion
	// steps, but it may save us some time once we get deeper.
	d := ol.Len() - olen
	if d < 0 {
		d = -d
	}

	if d > (c.blknum - idx)*(Nerrdata + Nerrmd) {
		return -1
	}

	// try without errors
	err = c.tryMd(idx, ol.Slice(13, 17), ol.Slice(17, 0), olen - 17, mdblks, data, difficulty)
	if err >= 0 {
		// we try to decode the data block only if we didn't assume it have errors
		if ol.Len() < 17 {
			// if the block is too short, don't even try to decode it
			return
		}

		v, errr := l0.Decode(prefix, ol.Slice(0, 17), c.crit)
		if errr != nil {
			return
		}

		pbit := int(v & 1)
		v >>= 1
		if (bits.OnesCount64(v) + pbit) % 2 == 0 {
			d := make([]byte, 4)
			d[0] = byte(v)
			d[1] = byte(v >> 8)
			d[2] = byte(v >> 16)
			d[3] = byte(v >> 24)
			data[idx] = d
		}

		return
	}

	// iterate through all possible errors
	for derr := 1; derr < Nerrdata; derr++ {
		// data deletes
		// (we assume that the deletes are not in the last derr nts)
		// TODO: We should calculate the prefix correctly by assuming there are
		// errors
		prefix := ol.Slice(13 - derr, 17 - derr)
		err = c.tryMd(idx, prefix, ol.Slice(17 - derr, 0), olen - 17, mdblks, data, difficulty)
		if err >= 0 {
			err += derr
			break
		}

		// data inserts
		// TODO: We should calculate the prefix correctly by assuming there are
		// errors
		prefix = ol.Slice(13 + derr, 17 + derr)
		err = c.tryMd(idx, prefix, ol.Slice(17 + derr, 0), olen - 17, mdblks, data, difficulty)
		if err >= 0 {
			err += derr
			break
		}
	}

	return
}

// This function gets the oligo with the data block stripped, so it starts with
// the metadata block. We try all possible error combinations that we allow in 
// the metadata block, store it's content in mdblks and recursively call 
// tryMatch to match the next blocks (if we haven't reached the end of the oligo)
// If we reached the end, we check if the metadata blocks we collected match
// the RS codes, and return the appropriate value.
func (c *Codec) tryMd(idx int, prefix, ol oligo.Oligo, olen int, mdblks []uint64, data [][]byte, difficulty int) (err int) {
	// TODO: this works for only single error allowed at the moment
	// should probably be made more general

	mdsz := c.mdsz
	if idx >= c.blknum - c.rsnum {
		mdsz = 5	// RS erasure blocks are always 5 nts
	}

	if ol.Len() < mdsz {
		return -1
	}

	// No error
	err = c.tryIt(idx, prefix, ol.Slice(0, mdsz), ol.Slice(mdsz, 0), olen - mdsz, mdblks, data, difficulty)
	if err >= 0 {
		return
	}

	// FIXME: The code below is not fully tested yet

	// One error
	// Delete
	// Iterate through all positions, and insert all possible nts
	if difficulty > 2 && ol.Len() + 1 >= mdsz {
		for p := 0; p < mdsz - 1; p++ {
			var sol oligo.Oligo
			if p == 0 {
				sol = long.New(0)
			} else {
				sol = ol.Slice(0, p)
			}
			eol := ol.Slice(p + 1, mdsz)

			for n := 0; n < len(singleNts); n++ {
				mdol, _ := long.Copy(sol)
				mdol.Append(singleNts[n])
				mdol.Append(eol)

				err = c.tryIt(idx, prefix, mdol, ol.Slice(mdsz - 1, 0), olen - mdsz, mdblks, data, difficulty)
				if err >= 0 {
					err++
					return
				}
			}
		}
	}

	// Insert
	// Iterate through all positions and remove one nt
	if difficulty > 1 && ol.Len() > mdsz {
		for p := 0; p < mdsz + 1; p++ {
			var mdol oligo.Oligo

			if p == 0 {
				mdol = long.New(0)
			} else {
				mdol = ol.Slice(0, p)
			}

			mdol.Append(ol.Slice(p+1, mdsz + 1))
			err = c.tryIt(idx, prefix, mdol, ol.Slice(mdsz + 1, 0), olen - mdsz, mdblks, data, difficulty)
			if err >= 0 {
				err++
				return
			}
		}
	}

	// Substitution
	// Iterate through all positions and replace the nt with the rest of the possible values
	if difficulty > 2 && ol.Len() >= mdsz {
		for p := 0; p < mdsz; p++ {
			var sol oligo.Oligo
			if p == 0 {
				sol = long.New(0)
			} else {
				sol = ol.Slice(0, p)
			}
			eol := ol.Slice(p + 1, mdsz)
			nt := ol.At(p)
			for n := 0; n < len(singleNts); n++ {
				if n == nt {
					continue
				}

				mdol, _ := long.Copy(sol)
				mdol.Append(singleNts[n])
				mdol.Append(eol)

				err = c.tryIt(idx, prefix, mdol, ol.Slice(mdsz, 0), olen - mdsz, mdblks, data, difficulty)
				if err >= 0 {
					err++
					return
				}
			}
		}
	}

	// we've gone through all and no luck
	return -1
}

func (c *Codec) tryIt(idx int, prefix, mdol, ol oligo.Oligo, olen int, mdblks []uint64, data [][]byte, difficulty int) (err int) {
	if idx+1 == c.blknum && olen != 0 {
		return -1
	}

	// if olen is negative, that means that the errors that we assumed earlier don't match to 
	// the oligo length that we expect. So we don't have a match
	if olen < 0 {
		return -1
	}

	md, errr := l0.Decode(prefix, mdol, c.crit)
	if errr != nil {
		return -1
	}

	// some common sense checks: if md is larger than the maximum value we
	// store per md block, it's probably wrong
	if idx < c.blknum - c.rsnum {
		if md > uint64(maxvals[c.mdsz]) {
			return -1
		}
	}

	mdblks[idx] = md
	if idx + 1 < c.blknum {
		// we still have a way to go before can check for match
		pfx := mdol
		if pfx.Len() > 4 {
			pfx = pfx.Slice(pfx.Len() - 4, pfx.Len())
		} else if pfx.Len() < 4 {
			pfx = prefix.Slice(prefix.Len() - 4 + pfx.Len(), prefix.Len())
			pfx.Append(mdol)
		}

		err = c.tryMatch(idx + 1, pfx, ol, olen, mdblks, data, difficulty)
		return
	}

	// we reached the end, check if it's a match
	mdshards := make([][]byte, len(mdblks))
	for i := 0; i < len(mdshards); i++ {
		mdshards[i] = append(mdshards[i], byte(mdblks[i]))
	}

	if ok, errr := c.ec.Verify(mdshards); !ok || errr != nil {
		err = -1
		return
	}

	// match!
	return 0	
}
