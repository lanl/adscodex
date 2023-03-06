// Package criteria defines which oligo sequences are acceptable 
// and can be synthesized and sequenced.
package criteria

import (
	"fmt"
	"adscodex/oligo"
)

type Criteria interface {
	// Unique identifier for the criteria. Only the low 48 bits should be used
	Id() uint64

	// Length of the features the criteria checks
	// For example, if the criteria checks for homopolymers of length 4, it
	// should return 4.
	// If the criteria doesn't have specific feature length, it should return -1.
	FeatureLength() int

	// Textual ID of the criteria
	String() string

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

func FindById(id uint64) Criteria {
	for _, c := range criterias {
		if c.Id() == id {
			return c
		}
	}

	return nil
}

// This function is called when the packages is used
func init() {
	Register("h4g2", H4G2)
	Register("h4-2", H4_2)
	Register("h4/2", H4D2)
	Register("h4g1", H4G1)
	Register("h4", H4)
}
