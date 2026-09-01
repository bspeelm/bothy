package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
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

// The two shapes projects publish in, and one field covers both: a sibling
// beside each asset, or one manifest for the whole release. What is inside is
// the same either way, so the parser does not need to know which it got.
func TestChecksumFileRendersBothShapes(t *testing.T) {
	p := platform.Info{OS: "linux", Arch: "x86_64"}
	for _, tc := range []struct {
		name      string
		checksums string
		want      string
	}{
		{"sibling", "{asset}.sha256", "thing-1.2.3-linux.tar.gz.sha256"},
		{"manifest", "checksums.txt", "checksums.txt"},
		{"manifest with a version", "t_{version}_checksums.txt", "t_1.2.3_checksums.txt"},
		{"publishes none", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := tools.Tool{
				Name:      "thing",
				Checksums: tc.checksums,
				Assets:    map[string]string{"linux_x86_64": "thing-{version}-linux.tar.gz"},
			}
			got, err := tool.ChecksumFile(p, "1.2.3")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("ChecksumFile() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Parsing the published file. A manifest lists every asset in the release, so
// picking the wrong line is how a checksum comes to be compared against the
// wrong artifact.
func TestUpstreamSumFindsTheRightLine(t *testing.T) {
	const asset = "rg-1.0-linux.tar.gz"
	manifest := "" +
		"aaaa  rg-1.0-darwin.tar.gz\n" +
		"bbbb  " + asset + "\n" +
		"cccc  rg-1.0-windows.zip\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, manifest)
	}))
	defer srv.Close()
	old := ReleaseBase
	ReleaseBase = srv.URL
	defer func() { ReleaseBase = old }()

	tool := tools.Tool{
		Name: "rg", Repo: "a/b", Checksums: "checksums.txt",
		Assets: map[string]string{"linux_x86_64": "rg-{version}-linux.tar.gz"},
	}
	got, err := upstreamSum(tool, platform.Info{OS: "linux", Arch: "x86_64"}, "v1.0", "1.0", asset)
	if err != nil {
		t.Fatal(err)
	}
	if got != "bbbb" {
		t.Errorf("upstreamSum() = %q, want the line for %s", got, asset)
	}
}

// A tool that publishes nothing must be silent, not an error -- four of the
// nine publish nothing and that is not a fault.
func TestUpstreamSumIsSilentWhenNothingIsPublished(t *testing.T) {
	tool := tools.Tool{Name: "delta", Repo: "a/b",
		Assets: map[string]string{"linux_x86_64": "delta.tar.gz"}}
	got, err := upstreamSum(tool, platform.Info{OS: "linux", Arch: "x86_64"}, "v1", "1", "delta.tar.gz")
	if err != nil || got != "" {
		t.Errorf("upstreamSum() = %q, %v; want silence", got, err)
	}
}

// A file that does not mention the asset is a broken assumption, not a match.
func TestUpstreamSumErrorsWhenTheAssetIsAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "aaaa  something-else.tar.gz\n")
	}))
	defer srv.Close()
	old := ReleaseBase
	ReleaseBase = srv.URL
	defer func() { ReleaseBase = old }()

	tool := tools.Tool{Name: "t", Repo: "a/b", Checksums: "checksums.txt",
		Assets: map[string]string{"linux_x86_64": "t.tar.gz"}}
	if _, err := upstreamSum(tool, platform.Info{OS: "linux", Arch: "x86_64"}, "v1", "1", "t.tar.gz"); err == nil {
		t.Error("a checksum file that does not list the asset was accepted")
	}
}

// The version shapes bothy actually reports, per install path. Getting this
// wrong announces an upgrade to someone who is already past it: a git-describe
// version is *ahead* of the tag it names, and Update.Outdated compares by
// string inequality rather than by ordering.
func TestIsSourceBuildKnowsWhichVersionsAreAhead(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		want          bool
	}{
		{"goreleaser release", "0.1.5", false},
		{"rpm spec", "0.1.5", false},
		{"go install via build info", "0.1.5", false},
		{"ldflags keeping its v", "v0.1.5", false},
		{"bare go build", "dev", true},
		{"empty", "", true},
		{"make install-binary at a tag", "v0.1.5-dirty", true},
		{"make install-binary past a tag", "v0.1.5-3-gabc1234", true},
		{"make install-binary past and dirty", "v0.1.5-3-gabc1234-dirty", true},
		// A release whose version merely contains a hyphen is not a source
		// build -- the marker is the git object, not the punctuation.
		{"a prerelease tag", "1.0.0-rc.1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSourceBuild(tc.version); got != tc.want {
				t.Errorf("IsSourceBuild(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}
