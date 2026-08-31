package fetch

import (
	"testing"

	"github.com/bspeelm/bothy/internal/tools"
)

// The lockfile is what makes an install reproducible and a tampered release a
// hard failure. A gap in it is not caught by any other test, because the code
// path that would notice only runs when something is actually being fetched.
func TestShippedLockCoversEveryToolOnLinux(t *testing.T) {
	lock, err := LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	all, err := tools.Load()
	if err != nil {
		t.Fatal(err)
	}

	for _, tool := range all {
		e, ok := lock.Get(tool.Name)
		if !ok {
			t.Errorf("%s is not pinned in bothy.lock — an install would refuse to fetch it", tool.Name)
			continue
		}
		if e.Version == "" || e.Tag == "" {
			t.Errorf("%s: tag=%q version=%q", tool.Name, e.Tag, e.Version)
		}
		// linux/x86_64 is the platform bothy is developed and tested on; a
		// missing checksum there means nobody has run this path.
		for _, p := range Platforms {
			if p.OS != "linux" {
				continue
			}
			if e.SHA(p) == "" {
				t.Errorf("%s has no checksum for %s_%s", tool.Name, p.OS, p.Arch)
			}
		}
	}
}

func TestLockChecksumsLookLikeChecksums(t *testing.T) {
	lock, err := LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range lock.Entries {
		for plat, sum := range e.SHA256 {
			if len(sum) != 64 {
				t.Errorf("%s/%s: %q is not a sha256", e.Name, plat, sum)
			}
			for _, c := range sum {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("%s/%s: %q is not lower-case hex", e.Name, plat, sum)
					break
				}
			}
		}
	}
}

// The pinned zellij must satisfy zellij's own minimum, or bothy would fetch a
// binary and then still report image previews as unavailable.
func TestPinnedVersionsSatisfyTheirOwnMinimums(t *testing.T) {
	lock, err := LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	all, err := tools.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range all {
		e, ok := lock.Get(tool.Name)
		if !ok {
			continue
		}
		min, err := tool.Min()
		if err != nil {
			continue
		}
		got, err := parseLockVersion(e.Version)
		if err != nil {
			t.Errorf("%s: version %q unparseable", tool.Name, e.Version)
			continue
		}
		if got.Less(min) {
			t.Errorf("%s pins %s, below its own minimum %s — fetching it would not help",
				tool.Name, got, min)
		}
	}
}
