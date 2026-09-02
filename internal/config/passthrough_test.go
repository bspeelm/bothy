package config

import (
	"slices"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/slots"
)

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

// The slot check is only as good as its vocabulary: a provider declaring a
// slot this package has never heard of would be silently unenforceable.
func TestEveryDeclaredSlotIsASlot(t *testing.T) {
	all, err := slots.All()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range all {
		if h.Slot == "" {
			continue
		}
		if !slices.Contains(slotNames, h.Slot) {
			t.Errorf("%s declares slot %q, which is not one of %v", h.Name, h.Slot, slotNames)
		}
	}
}

// bothy config set slots.mux yazi used to be accepted in silence, along with
// every other assignment of a provider to a slot it cannot fill -- three
// commands and a workspace that cannot open. Nothing in the data said which
// slot yazi filled, so there was nothing to be wrong against.
func TestASlotRejectsAProviderThatFillsAnother(t *testing.T) {
	for _, tc := range []struct{ key, value, want string }{
		{"slots.mux", "yazi", "fills the browser slot"},
		{"slots.browser", "zellij", "fills the mux slot"},
		{"slots.agent", "ghostty", "fills the terminal slot"},
		{"slots.mux", "fzf", "fills no slot"},
	} {
		c := Default()
		err := c.Set(tc.key, tc.value)
		if err == nil {
			t.Errorf("Set(%q, %q) was accepted", tc.key, tc.value)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Set(%q, %q) = %v, want it to say %q", tc.key, tc.value, err, tc.want)
		}
	}
}

// The rule catches a contradiction and otherwise stays out of the way. The
// README promises the agent slot takes any command you care to name, and the
// terminal slot names emulators bothy ships no file for.
func TestASlotStillTakesANameBothyHasNoFileFor(t *testing.T) {
	for _, tc := range [][2]string{
		{"slots.agent", "my-own-thing"},
		{"slots.terminal", "kitty"},
		{"slots.terminal", "wezterm"},
		{"slots.browser", "none"},
		{"slots.editor", "helix"},
		{"slots.mux", ""},
	} {
		c := Default()
		if err := c.Set(tc[0], tc[1]); err != nil {
			t.Errorf("Set(%q, %q) = %v, want it accepted", tc[0], tc[1], err)
		}
	}
}

// Validate has to reach the same verdict as Set, for a config.toml written by
// hand rather than typed at bothy config set.
func TestValidateRejectsAMismatchedSlot(t *testing.T) {
	c := Default()
	c.Slots.Mux = "yazi"
	if err := c.Validate(); err == nil {
		t.Error("Validate accepted mux = yazi")
	}
	if err := Default().Validate(); err != nil {
		t.Errorf("Validate rejected the default config: %v", err)
	}
}
