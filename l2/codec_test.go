package l2

import (
	"flag"
	"fmt"
	"os"
	"math/rand"
	"testing"
	"time"
	"adscodex/oligo"
	"adscodex/oligo/long"
	"adscodex/l1"
	"adscodex/utils/errmdl/simple"
)

var dseqnum = flag.Int("dseqnum", 3, "number of data oligos in an erasure group")
var eseqnum = flag.Int("rseqnum", 2, "number of erasure oligos in an erasure group")
var dbnum = flag.Int("dbnum", 5, "number of data blocks")
var mdsz = flag.Int("mdsz", 4, "metadata block size")
var mdcnum = flag.Int("mdcnum", 2, "metadata error detection blocks")
var mdctype = flag.String("mdctype", "rs", "metadata error detection type (rs or crc)")
var dtctype = flag.String("dtctype", "parity", "data error detection type (parity or even)")
var ierrate = flag.Float64("ierr", 0.034, "error rate (percent)")
var derrate = flag.Float64("derr", 1.084, "error rate (percent)")
var serrate = flag.Float64("serr", 0.752, "error rate (percent)")
var prob = flag.Float64("prob", 0.8,  "probability for negative binomial distribution")
var dfclty =  flag.Int("dfclty", 1, "decoding difficulty level")
var crit = flag.String("crit", "h4g2", "criteria")
var seed = flag.Int64("s", 1, "random generator seed")
var depth = flag.Int("depth", 30, "depth")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

type Stat struct {
	count	int		// number of oligos
	size	uint64		// number of bytes en/decoded
	dur	int64		// time in milliseconds to test
	extra	int		// number of bytes out of range
	verfp	uint64		// number of false positives in verified data
	uverfp	uint64		// number of false positives in unverified data
	versz	uint64		// number of verified bytes
	uversz	uint64		// number of unverified bytes
	holesz	uint64		// number of missing bytes
	errnum	int		// number of errors introduced
	readnum	int		// number of reads
	failnum int		// number of oligos that failed
	vmulti	int		// multiple verified 
}

var p5, p3 oligo.Oligo
var cdc *Codec
var rndseed int64
var errmdl *simple.SimpleErrorModel
var rnd *rand.Rand

func initTest(t *testing.T) {
	var err error

	if cdc != nil {
		return
	}

	p5, _ = long.FromString("CGACATCTCGATGGCAGCA")
	p3, _ = long.FromString("ATCAGTGAGCTGGCAACTTCCA")
	cdc, err = NewCodec(p5, p3, *dbnum, *mdsz, *mdcnum, *dseqnum, *eseqnum)
	if err != nil {
		t.Fatalf("%v\n", err)
	}

	switch *mdctype {
	default:
		err = fmt.Errorf("Error: invalid metadata EC type")

	case "crc":
		err = cdc.SetMetadataChecksum(l1.CSumCRC)

	case "rs":
		err = cdc.SetMetadataChecksum(l1.CSumRS)
	}

	switch *dtctype {
	default:
		err = fmt.Errorf("Error: invalid data EC type")

	case "parity":
		err = cdc.SetDataChecksum(l1.CSumParity)

	case "even":
		err = cdc.SetDataChecksum(l1.CSumEven)
	}

	if err != nil {
		t.Fatalf("%v\n", err)
	}

	if *ierrate + *derrate + *serrate > 100 {
		t.Fatalf("Total error rate can't be more than 100%%\n")
	}

	rndseed = *seed
	if rndseed == 0 {
		rndseed = time.Now().UnixNano()
	}
	errmdl = simple.New(*ierrate/100, *derrate/100, *serrate/100, *prob, rndseed)
	rnd = rand.New(rand.NewSource(rndseed))
}

func TestEncode(t *testing.T) {
	var ne Stat

	initTest(t)

	fmt.Printf("TestEncode:\n")
	for n := 0; n < 1; n++ {
		sz := rand.Intn(1<<16)
		data := make([]byte, sz)
		for i := 0; i < sz; i++ {
			data[i] = byte(rand.Intn(256))
		}

		fmt.Printf("--------------- %d bytes ------------\n", sz)
		nextaddr, oligos, err := cdc.Encode(0, data)
		if err != nil {
			t.Fatalf("encode error: %v\n", err)
		}

		// add some errors
		nols, nerr := errmdl.GenMany(*depth * len(oligos), oligos)
		ne.errnum += nerr
		ne.readnum += len(nols)

		omap := make(map[string]int)
		for _, o := range nols {
			omap[o.String()]++
		}
		
		fmt.Printf("\nDecoding orig %d with errors %d oligos %d unique...\n", len(oligos), len(nols), len(omap))
		de := cdc.Decode(0, nextaddr, nols)
		for i := 0; i < len(de); i++ {
			fmt.Printf("\textent %d offset %d size %d verified %v\n", i, de[i].Offset, len(de[i].Data), de[i].Verified)
		}
/*
		if len(de) == 0 {
			t.Fatalf("decode error no data\n")
		} else if len(de) != 1 {
			t.Fatalf("decode error: too many data extents: %d\n", len(de))
		}

		ddata := de[0].Data
		if len(data) != len(ddata) {
			t.Fatalf("decoded data length doesn't match: %d %d\n", len(data), len(ddata))
		}

		for i := 0; i < len(data); i++ {
			if data[i] != ddata[i] {
				fmt.Printf("%v\n", data)
				fmt.Printf("%v\n", ddata)
				t.Fatalf("decoded data doesn't match")
			}
		}
*/
	}
}

func TestEcGroup(t *testing.T) {
	var ne, oe Stat

	return
	initTest(t)
	ecsz := ecGroupDataSize(4, *dbnum, *dseqnum)
	data := make([]byte, ecsz)
//	dpr := make([]bool, len(data))
	dblks := make([]Blk, *dbnum)

	for iter := 0; iter < 1000; iter++ {
		// generate some random data
		for i := 0; i < len(data); i++ {
			data[i] = byte(rnd.Intn(256))
		}

		addr := uint64(rnd.Int63n(int64(cdc.MaxAddr())))
		addr -= addr%uint64(*dseqnum)						// make sure the oligos are aligned and are a single ECG

		rows, err := ecGroupEncode(4, *dbnum, *dseqnum, *eseqnum, cdc.ec, data)
		if err != nil {
			t.Fatalf("%v", err)
		}

		var ols []oligo.Oligo
		for r, row := range rows {
//			fmt.Printf("%v\n", row)
			a := addr + uint64(r)
			e := false
			if r >= *dseqnum {
				a -= uint64(*dseqnum)
				e = true
			}

			ol, err := cdc.c1.Encode(p5, p3, a, e, row)
			if err != nil {
				t.Fatalf("%v", err)
			}

			ols = append(ols, ol)
		}

		// add some errors
		nols, nerr := errmdl.GenMany(*depth * len(ols), ols)
		ne.errnum += nerr
		oe.errnum += nerr
		ne.readnum += len(nols)
		oe.readnum += len(nols)

		// First try the new code
		eg := newEcGroup(*dseqnum + *eseqnum, *dbnum)
		for _, ol := range nols {
//			fmt.Printf("%v: ", ol)
			daddr, def, ddata, err := cdc.c1.Decode(p5, p3, ol, *dfclty)
			if err != nil {
				// we couldn't decode, ignore
				if err != l1.Eprimer {
					ne.failnum++
				}

				continue
			}

			if daddr < addr || daddr >= addr+uint64(*dseqnum) {
				// the address doesn't match, ignore
				ne.extra += len(ddata)
				continue
			}

			row := daddr - addr
			if def {
				row += uint64(*dseqnum)
				if row >= uint64(*dseqnum + *eseqnum) {
					ne.extra += len(ddata)
					continue
				}
			}

			for i, d := range ddata {
				dblks[i] = Blk(d)
			}

			eg.addEntry(int(row), dblks, *eseqnum, cdc.ec)
		}

		for r := 0; r < *dseqnum; r++ {
			for c := 0; c < *dbnum; c++ {
				ne.size += 4
				start := (r * *dbnum + c) * 4
				vd := eg.getVerified(r, c)
//				fmt.Printf("** (%d, %d): %d\n", r, c, len(vd))
				switch len(vd) {
				case 0:
					// nothing for now, it will be handled at the unverfified stage
				case 1:
					if len(vd[0]) != 4 {
						panic(fmt.Sprintf("nooo %d", len(vd)))
					}

					for i, v := range vd[0] {
						if v != data[start + i] {
//							fmt.Printf("(%d, %d):%d expected %d got %d\n", r, c, start + i, data[start + i], v)
							ne.verfp++
						} else {
							ne.versz++
						}
					}
					continue

				default:
					// too many copies, assume unverified
					ne.uversz += 4
					ne.vmulti += 4
					continue
				}

				ud := eg.getUnverified(r, c)
				switch len(ud) {
				case 0:
					ne.holesz += 4

				case 1:
					for i, v := range ud[0] {
						if v != data[start + i] {
							ne.uverfp++
						} else {
							ne.uversz++
						}
					}
					continue

				default:
					// too many copies, assume unverified
					ne.uversz += 4
					continue
				}
			}
		}

		if ne.size != ne.uversz + ne.uverfp + ne.versz + ne.verfp + ne.holesz {
			panic("boo")
		}

/*
		// Then compare it to the old code
		dss, failed := cdc.DecodeECG(*dfclty, nols)
		oe.failnum += failed
		for i := 0; i < len(dpr); i++ {
			dpr[i] = false
		}

		offset := addr * uint64((4 * *dbnum))	// TODO: fix this to use the codec provided values
//		fmt.Printf("---\n")
		for _, ds := range dss {
//			fmt.Printf("%d %d\n", ds.Offset - offset, len(ds.Data))
			if ds.Offset < offset || ds.Offset >= offset+uint64(len(data)) {
//				fmt.Printf("Extra! %d %d\n", addr, ds.Offset)
				// the data is completely out the range
				oe.extra += len(ds.Data)
				continue
			}

			idx := int(ds.Offset - offset)
			for i := 0; i < len(ds.Data); i++ {
				// shouldn't happen, but these things happen all the time
				if dpr[idx + i] {
					panic("overlapping DataExtent entries")
				}

				dpr[idx + i] = true
			}

			if ds.Verified {
//				fmt.Printf("Verified!\n")
				// check if the verified data is correct
//				if ne.verfp != 0 {
//					fmt.Printf("oe %d: %v\n", idx, ds.Data)
//				}

				for i := 0; i < len(ds.Data); i++ {
					if ds.Data[i] != data[i+idx] {
						oe.verfp++
					}
				}

				oe.versz += uint64(len(ds.Data))
			} else {
//				fmt.Printf("Unverified!\n")
				// check if the unverified data is correct
				for i := 0; i < len(ds.Data); i++ {
					if ds.Data[i] != data[i+idx] {
						oe.uverfp++
					}
				}

				oe.uversz += uint64(len(ds.Data))
			}
		}

		// check for missing data
		for _, f := range dpr {
			if !f {
				oe.holesz++
			}
		}

		oe.size += uint64(len(data))
*/
	}

	fmt.Printf("new stat %v\n", ne)
//	fmt.Printf("old stat %v\n", oe)
}

func (st Stat) String() string {
	return fmt.Sprintf("size %d extra %d verfp %d uverfp %d versz %d uversz %d hole %d failed %d vmulti %d", st.size, st.extra, st.verfp, st.uverfp, st.versz, st.uversz, st.holesz, st.failnum, st.vmulti)
}
