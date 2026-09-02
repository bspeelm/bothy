package config

import "testing"

// #69. passthrough is a documented feature that did not work in either
// spelling: Validate demanded slot names, every caller asked by provider name,
// so ["yazi"] was rejected by `config set` and honoured at install, while
// ["browser"] was accepted and silently ignored.
//
// Nobody wrote a bug. Two people wrote the obvious thing at different times,
// and there was nothing to be wrong against, because nothing in the data says
// yazi fills browser.
func TestBothSpellingsOfPassthroughWork(t *testing.T) {
	for _, spelling := range []string{"browser", "yazi"} {
		c := Default()
		c.Passthrough = []string{spelling}

		if !c.PassesThrough("browser") {
			t.Errorf("passthrough = [%q] does not pass the browser slot through", spelling)
		}
		if err := c.Validate(); err != nil {
			t.Errorf("passthrough = [%q] is rejected: %v", spelling, err)
		}
	}
}

// The provider spelling follows the slot: naming the provider passes through
// the slot it is in, and stops meaning anything once something else is in it.
func TestTheProviderSpellingFollowsTheSlot(t *testing.T) {
	c := Default()
	c.Passthrough = []string{"yazi"}
	c.Slots.Browser = "none"

	if c.PassesThrough("browser") {
		t.Error(`passthrough = ["yazi"] still passes the browser slot through ` +
			`after the browser was changed; a provider name only means the slot it fills`)
	}
}

// A name that is neither is still an error, or the check stops checking.
func TestPassthroughStillRejectsNonsense(t *testing.T) {
	c := Default()
	c.Passthrough = []string{"emacs"}
	if err := c.Validate(); err == nil {
		t.Error("passthrough accepted a name that is neither a slot nor a configured provider")
	}
}

// ProviderFor reads the slot names off the struct tags, so a slot added to
// Slots is answerable here without a second list to keep in step.
func TestProviderForCoversEverySlot(t *testing.T) {
	c := Default()
	for _, slot := range slotNames {
		if c.ProviderFor(slot) == "" {
			t.Errorf("ProviderFor(%q) is empty, but Default() fills every slot", slot)
		}
	}
	if got := c.ProviderFor("not-a-slot"); got != "" {
		t.Errorf("ProviderFor(\"not-a-slot\") = %q, want empty", got)
	}
}
