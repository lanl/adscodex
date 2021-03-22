package errmdl

import (
	"acoma/oligo"
)

type ErrMdl interface {
	// Generate one "read" of the specified oligo based on the error
	// model for a single oligo.
	GenOne(ol oligo.Oligo) (r oligo.Oligo, errnum int)

	// Generate readnum "reads" from the specified array of oligos.
	// It uses both the error model for a single oligo as well as the
	// abundance error model.
	// Returns an array with numreads elements.
	GenMany(numreads int, ols []oligo.Oligo) (rols []oligo.Oligo, errnum int)
}
