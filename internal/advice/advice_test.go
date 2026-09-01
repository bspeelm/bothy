package advice

import (
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
)

// An image-based host needs a different command and a reboot. A generic
// "install ghostty" leaves exactly the person with the hardest job to work it
// out alone.
func TestImmutableHostGetsItsOwnCommand(t *testing.T) {
	a, err := Get("ghostty")
	if err != nil {
		t.Fatal(err)
	}
	ostree := a.Command(platform.Info{OS: "linux", DistroID: "fedora", Immutable: true})
	plain := a.Command(platform.Info{OS: "linux", DistroID: "fedora"})

	if ostree == plain {
		t.Fatal("an ostree host got the same command as a mutable one")
	}
	if !strings.Contains(ostree, "rpm-ostree") {
		t.Errorf("ostree command does not use rpm-ostree: %s", ostree)
	}
	if !strings.Contains(ostree, "reboot") {
		t.Error("the ostree command should say a reboot is part of it")
	}
	if strings.Contains(plain, "rpm-ostree") {
		t.Errorf("a mutable Fedora was told to use rpm-ostree: %s", plain)
	}
}

// The repository warnings are the expensive part of this knowledge: one of them
// blocks system upgrades until it is removed.
func TestGhosttyAdviceCarriesTheRepositoryWarnings(t *testing.T) {
	a, err := Get("ghostty")
	if err != nil {
		t.Fatal(err)
	}
	w := a.Warnings(platform.Info{OS: "linux", DistroID: "fedora"})
	if !strings.Contains(w, "pgdev") {
		t.Error("the warning about the repo that blocks rpm-ostree upgrades is missing")
	}
	if !strings.Contains(w, "rpm-ostree") {
		t.Error("the warning should say what it actually breaks")
	}

	// Both warnings are about Copr repositories, so anywhere that cannot
	// enable one should hear nothing.
	if w := a.Warnings(platform.Info{OS: "linux", DistroID: "ubuntu"}); w != "" {
		t.Errorf("Ubuntu was warned about a Copr it cannot enable: %s", w)
	}
}

// Derivatives say which distribution to treat them as, and bothy should
// listen: without it Mint and Pop!_OS got the generic advice.
func TestDerivativesInheritTheirParentsAdvice(t *testing.T) {
	a, err := Get("ghostty")
	if err != nil {
		t.Fatal(err)
	}
	mint := a.Command(platform.Info{OS: "linux", DistroID: "linuxmint", DistroLike: "ubuntu"})
	if mint != a.Command(platform.Info{OS: "linux", DistroID: "ubuntu"}) {
		t.Errorf("Mint did not inherit Ubuntu's advice: %s", mint)
	}
}

// An unknown platform must still say something useful rather than an empty fix.
func TestUnknownPlatformStillAdvises(t *testing.T) {
	a, _ := Get("ghostty")
	if got := a.Command(platform.Info{OS: "plan9", DistroID: "9front"}); got == "" {
		t.Error("no advice at all for an unrecognised platform")
	}
}

func TestAgentAdviceExists(t *testing.T) {
	a, err := Get("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if a.Command(platform.Info{OS: "linux"}) == "" {
		t.Error("no install command for the agent")
	}
}

// #72. The name a provider is configured by is not always the command it runs
// as -- helix is hx, claude-code is claude -- and that mapping lived in an
// EditorBinary switch, an agentBinary switch, and a map inlined in a doctor
// check. Three copies in two packages, beside an advice.binary field that was
// parsed and never read.
func TestBinaryComesFromTheProvidersOwnFile(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"helix", "hx"},
		{"neovim", "nvim"},
		{"claude-code", "claude"},
		{"gemini-cli", "gemini"},
		// Named the same as the command it runs, and said so in its own file.
		{"vim", "vim"},
		// No advice file at all: a provider is run as its own name, which is
		// true of most of them and is what makes "any command you name" work.
		{"emacs", "emacs"},
	} {
		if got := Binary(tc.name); got != tc.want {
			t.Errorf("Binary(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// Every advised provider whose command differs from its name has to say so,
// or the difference goes back to being knowledge held in Go.
func TestAdvisedProvidersDeclareTheirBinary(t *testing.T) {
	for _, name := range []string{"helix", "neovim", "claude-code", "gemini-cli"} {
		a, err := Get(name)
		if err != nil {
			t.Errorf("no advice file for %q: %v", name, err)
			continue
		}
		if a.Binary == "" {
			t.Errorf("%s declares no binary, so bothy would run a command by that name", name)
		}
	}
}
