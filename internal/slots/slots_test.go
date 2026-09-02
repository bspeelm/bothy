package slots

import (
	"slices"
	"testing"

	bothy "github.com/bspeelm/bothy"
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

// A name resolves to exactly one provider, whichever way it is obtained.
func TestGetReachesEveryProvider(t *testing.T) {
	for _, tc := range []struct{ name, slot string }{
		{"zellij", "mux"},       // [fetch]
		{"yazi", "browser"},     // [fetch]
		{"ghostty", "terminal"}, // [advise]
		{"vim", "editor"},       // [advise]
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

// A template path that does not exist fails at install time, on someone else's
// machine, having already written the files listed before it. The embed can be
// checked here instead.
func TestEveryTemplateAFileNamesExists(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, pr := range all {
		for _, f := range pr.Files {
			if f.Template == "" || f.Dest == "" {
				t.Errorf("%s has a [[file]] missing template or dest", pr.Name)
				continue
			}
			if _, err := bothy.Templates.ReadFile(f.Template); err != nil {
				t.Errorf("%s names template %q, which is not embedded", pr.Name, f.Template)
			}
			seen++
		}
	}
	if seen == 0 {
		t.Error("no provider generates any file, which cannot be right")
	}
}

// The conditional files are the ones a reader of install.plan() used to be
// able to see and now cannot, so the set is named here rather than trusted.
func TestOnlyTheKnownFilesAreConditional(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"yazi/plugins/enter-hint.yazi/main.lua": "no-images",
		"vim/vimrc":                             "provide-editor-config",
		"vim/colors/{theme}.vim":                "provide-editor-config",
	}
	for _, pr := range all {
		for _, f := range pr.Files {
			if f.When == "" {
				continue
			}
			if want[f.Dest] != f.When {
				t.Errorf("%s writes %s when %q; the known set says %q",
					pr.Name, f.Dest, f.When, want[f.Dest])
			}
			delete(want, f.Dest)
		}
	}
	for dest := range want {
		t.Errorf("%s was conditional and no longer is", dest)
	}
}

// Every provider is obtainable one way or the other, or bothy has a name for
// something it can neither install nor tell you how to install.
func TestEveryProviderCanBeObtained(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range all {
		if pr.Fetch == nil && pr.Advise == nil {
			t.Errorf("%s has neither [fetch] nor [advise]", pr.Name)
		}
	}
}
