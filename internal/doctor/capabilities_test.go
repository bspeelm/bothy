package doctor

import (
	"slices"
	"testing"

	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/slots"
)

// provides is a capability vocabulary shared between the data and the doctor.
// A provider claiming something outside it is a claim nothing can act on, and
// it would read as "unsupplied" rather than as the typo it is.
func TestEveryClaimedCapabilityIsOne(t *testing.T) {
	all, err := slots.All()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range all {
		for _, name := range h.Provides {
			if !slices.Contains(Capabilities, Capability(name)) {
				t.Errorf("%s claims %q, which is not a capability %v", h.Name, name, Capabilities)
			}
		}
	}
}

// The default stack is the one bothy ships and the one CI installs, so every
// capability it reports on has to have something behind it. Isolation is the
// exception by construction: it is bothy's own doing, not a provider's.
func TestTheDefaultStackSuppliesEveryCapability(t *testing.T) {
	supplied := Supplied(config.Default())
	for _, c := range Capabilities {
		if !supplied[c] {
			t.Errorf("nothing in the default stack contributes to %s", c)
		}
	}
}

// The negative direction is the one Supplied is sound in, and the one the
// capability report turns on: strip the stack and it has to say so.
func TestAStackWithNothingBehindACapabilitySaysSo(t *testing.T) {
	c := config.Default()
	c.Slots.Terminal = "kitty" // no file, so no claim
	c.Slots.Mux = "tmux"       // likewise
	c.Slots.Browser = "none"
	c.Slots.Editor = "nano"

	supplied := Supplied(c)
	for _, cap := range []Capability{Panes, Sessions, Images, Theme} {
		if supplied[cap] {
			t.Errorf("%s is reported as supplied by a stack that has nothing to do it", cap)
		}
	}
	if !supplied[Isolation] {
		t.Error("isolation is bothy's own doing and should survive any stack")
	}
}
