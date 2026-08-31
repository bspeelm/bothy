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
	w := a.Warnings()
	if !strings.Contains(w, "pgdev") {
		t.Error("the warning about the repo that blocks rpm-ostree upgrades is missing")
	}
	if !strings.Contains(w, "rpm-ostree") {
		t.Error("the warning should say what it actually breaks")
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
