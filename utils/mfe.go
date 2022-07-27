package utils

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"adscodex/oligo"
)

type mfeEntry struct {
	ol	oligo.Oligo
	mfe	float32
}

func CalculateMfe(ols []oligo.Oligo, temp float32) (mfes map[oligo.Oligo]float32, err error) {
	mfes = make(map[oligo.Oligo]float32)
	ch := make(chan oligo.Oligo)
	errch := make(chan error)
	olch := make(chan mfeEntry)
	nprocs := runtime.NumCPU()
	for i := 0; i < nprocs; i++ {
		go func() {
			var stdout io.ReadCloser
			var stdin io.WriteCloser
			var out *bufio.Reader
			var in *bufio.Writer
			var err error

			cmd := exec.Command("mfepipe", "-material", "dna", "-T", fmt.Sprintf("%f", temp))
			if stdin, err = cmd.StdinPipe(); err != nil {
				errch <- err
				return
			}

			if stdout, err = cmd.StdoutPipe(); err != nil {
				errch <- err
				return
			}

			if err := cmd.Start(); err != nil {
				errch <- err
				return
			}

			in = bufio.NewWriter(stdin)
			out = bufio.NewReader(stdout)

			for {
				ol := <- ch
				if ol == nil {
					break
				}

				var mfe float32
				fmt.Fprintf(in, "%v\n", ol)
				in.Flush()
				_, err := fmt.Fscanf(out, "%f\n", &mfe)
				if err != nil {
					errch <- err
				}

				olch <- mfeEntry{ ol, mfe }
			}

			if stdin != nil {
				stdin.Close()
			}

			if stdout != nil {
				stdout.Close()
			}
		}()
	}

	for i := 0; i < len(ols); {
		select {
		case ch <- ols[i]:
			i++

		case err = <-errch:
			nprocs--
			goto error

		case mol := <-olch:
			mfes[mol.ol] = mol.mfe
		}
	}

error:
	// signal procs to exit
	for i := 0; i < nprocs; {
		select {
		case ch <- nil:
			i++;

		case err = <-errch:
			nprocs--
			goto error

		case mol := <-olch:
			mfes[mol.ol] = mol.mfe
		}
	}

	return
}
