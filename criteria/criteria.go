// Package criteria defines which oligo sequences are acceptable 
// and can be synthesized and sequenced.
package criteria

import (
	"acoma/oligo"
)

type Criteria interface {
	// returns true if the oligo is acceptible
	Check(o oligo.Oligo) bool
}
