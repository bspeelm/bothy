package install

import (
	"sort"
	"testing"

	"github.com/bspeelm/bothy/internal/slots"
)

// A `when` nothing recognises writes the file, so a typo shows rather than
// silently dropping a config. That makes the data the only place the mistake
// can be caught, which is what this is for.
func TestEveryWhenIsAKnownCondition(t *testing.T) {
	all, err := slots.All()
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range all {
		for _, f := range pr.Files {
			if f.When == "" {
				continue
			}
			if _, ok := conditions[f.When]; !ok {
				var known []string
				for k := range conditions {
					known = append(known, k)
				}
				sort.Strings(known)
				t.Errorf("%s names condition %q; the vocabulary is %v",
					pr.Name, f.When, known)
			}
		}
	}
}
