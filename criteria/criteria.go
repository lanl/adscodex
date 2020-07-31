// Package criteria defines which oligo sequences are acceptable 
// and can be synthesized and sequenced.
package criteria

import (
	"fmt"
	"acoma/oligo"
)

type Criteria interface {
	// Unique identifier for the criteria. Only the low 48 bits should be used
	Id() uint64

	// Length of the features the criteria checks
	// For example, if the criteria checks for homopolymers of length 4, it
	// should return 4.
	// If the criteria doesn't have specific feature length, it should return -1.
	FeatureLength() int

	// returns true if the oligo is acceptible
	Check(o oligo.Oligo) bool
}

var criterias map[string] Criteria

func Register(name string, c Criteria) (err error) {
	if criterias == nil {
		criterias = make(map[string] Criteria)
	}

	if criterias[name] != nil {
		return fmt.Errorf("Criteria with name '%s' already registered", name)
	}

	criterias[name] = c
	return
}

func Find(name string) Criteria {
	return criterias[name]
}

// This function is called when the packages is used
func init() {
	Register("H4G2", H4G2)
}
