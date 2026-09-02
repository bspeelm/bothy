package slots

import (
	"slices"
	"testing"
)

// Every provider has to be findable and describable, because the header is
// what `bothy tools` prints and what the slot check reads. A file that
// declares neither is a provider nothing can say anything about.
func TestEveryProviderIsNamedAndDescribed(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 15 {
		t.Fatalf("read %d providers, expected both dialects", len(all))
	}
	for _, h := range all {
		if h.Name == "" {
			t.Errorf("a provider file declares no name")
		}
		if h.What == "" {
			t.Errorf("%s declares no what", h.Name)
		}
		if len(h.Platforms) == 0 {
			t.Errorf("%s declares no platforms", h.Name)
		}
	}
}

// The two dialects are disjoint on disk, and All joining them is the whole
// point of the package: a name must resolve to exactly one header whichever
// directory it lives in.
func TestGetReachesBothDialects(t *testing.T) {
	for _, tc := range []struct{ name, slot string }{
		{"zellij", "mux"},       // slots/tools
		{"yazi", "browser"},     // slots/tools
		{"ghostty", "terminal"}, // slots/advice
		{"vim", "editor"},       // slots/advice
		{"claude-code", "agent"},
	} {
		h, ok := Get(tc.name)
		if !ok {
			t.Errorf("Get(%q) found nothing", tc.name)
			continue
		}
		if h.Slot != tc.slot {
			t.Errorf("%s declares slot %q, want %q", tc.name, h.Slot, tc.slot)
		}
		if !slices.Contains(Fills(tc.slot), tc.name) {
			t.Errorf("Fills(%q) does not include %s", tc.slot, tc.name)
		}
	}
}

// An unknown name is not an error. The agent slot takes any command you care
// to name, so Get has to answer "no file" without failing.
func TestGetIsSilentAboutNamesItHasNoFileFor(t *testing.T) {
	if _, ok := Get("my-own-thing"); ok {
		t.Error("Get invented a header for a name with no file")
	}
	if got := Fills(""); got != nil {
		t.Errorf("Fills(\"\") = %v, want nil -- an extra fills no slot", got)
	}
}

// The extras are the reason Slot is allowed to be empty.
func TestExtrasFillNoSlot(t *testing.T) {
	for _, name := range []string{"fd", "fzf", "jq", "lazygit", "ripgrep", "zoxide"} {
		h, ok := Get(name)
		if !ok {
			t.Fatalf("%s has no file", name)
		}
		if h.Slot != "" {
			t.Errorf("%s declares slot %q, but it is an extra", name, h.Slot)
		}
	}
}
