package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
	"github.com/bspeelm/bothy/internal/tools"
	"strings"
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

// The shapes projects publish in, and one field covers them all: a sibling
// beside each asset, a sibling named for the asset with its archive extension
// swapped, or one manifest for the whole release. What is inside is the same
// either way, so the parser does not need to know which it got.
func TestChecksumFileRendersEveryShape(t *testing.T) {
	p := platform.Info{OS: "linux", Arch: "x86_64"}
	for _, tc := range []struct {
		name      string
		checksums string
		want      string
	}{
		{"sibling", "{asset}.sha256", "thing-1.2.3-linux.tar.gz.sha256"},
		{"sibling, extension swapped", "{asset_stem}.sha256sum", "thing-1.2.3-linux.sha256sum"},
		{"manifest", "checksums.txt", "checksums.txt"},
		{"manifest with a version", "t_{version}_checksums.txt", "t_1.2.3_checksums.txt"},
		{"publishes none", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := tools.Tool{Header: slots.Header{Name: "thing"}, Fetch: slots.Fetch{Checksums: tc.checksums, Assets: map[string]string{"linux_x86_64": "thing-{version}-linux.tar.gz"}}}
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

	tool := tools.Tool{Header: slots.Header{Name: "rg"}, Fetch: slots.Fetch{Repo: "a/b", Checksums: "checksums.txt", Assets: map[string]string{"linux_x86_64": "rg-{version}-linux.tar.gz"}}}
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
	tool := tools.Tool{Header: slots.Header{Name: "delta"}, Fetch: slots.Fetch{Repo: "a/b", Assets: map[string]string{"linux_x86_64": "delta.tar.gz"}}}
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

	tool := tools.Tool{Header: slots.Header{Name: "t"}, Fetch: slots.Fetch{Repo: "a/b", Checksums: "checksums.txt", Assets: map[string]string{"linux_x86_64": "t.tar.gz"}}}
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

// zellij publishes a checksum of the binary inside the archive, named by the
// path it was built at, so nothing in the file matches the asset. Matching on
// the basename is what makes that release cross-checkable at all; without it
// zellij's four platforms stay trust-on-first-use.
func TestUpstreamSumMatchesTheBinaryInsideTheArchive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "d006c521  target/x86_64-unknown-linux-musl/release/zellij\n")
	}))
	defer srv.Close()
	old := ReleaseBase
	ReleaseBase = srv.URL
	defer func() { ReleaseBase = old }()

	tool := tools.Tool{
		Header: slots.Header{Name: "zellij"},
		Fetch: slots.Fetch{
			Binary:         "zellij",
			Checksums:      "{asset_stem}.sha256sum",
			ChecksumCovers: "binary",
			Assets:         map[string]string{"linux_x86_64": "zellij-x86_64-unknown-linux-musl.tar.gz"},
		},
	}
	got, err := upstreamSum(tool, platform.Info{OS: "linux", Arch: "x86_64"},
		"v0.45.1", "0.45.1", "zellij-x86_64-unknown-linux-musl.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "d006c521" {
		t.Errorf("upstreamSum = %q, want the line naming the binary", got)
	}
}

// And a tool that has not opted in must not match on a basename: a manifest
// listing "bin/thing" alongside the asset would otherwise be read as the
// asset's checksum and compared against the wrong bytes.
func TestUpstreamSumWillNotMatchABasenameUnasked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "aaaa  bin/thing\n")
	}))
	defer srv.Close()
	old := ReleaseBase
	ReleaseBase = srv.URL
	defer func() { ReleaseBase = old }()

	tool := tools.Tool{
		Header: slots.Header{Name: "thing"},
		Fetch: slots.Fetch{
			Binary:    "thing",
			Checksums: "sums.txt",
			Assets:    map[string]string{"linux_x86_64": "thing-linux.tar.gz"},
		},
	}
	if got, err := upstreamSum(tool, platform.Info{OS: "linux", Arch: "x86_64"},
		"v1", "1", "thing-linux.tar.gz"); err == nil {
		t.Errorf("upstreamSum = %q, want a refusal: nothing in the file names the asset", got)
	}
}

// The stem is how a sibling checksum is named for an archive. Getting it wrong
// yields a 404, which reads as "no upstream checksum published" -- a silent
// downgrade to trust-on-first-use rather than an error.
func TestAssetStemDropsOnlyTheArchiveExtension(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"zellij-x86_64-unknown-linux-musl.tar.gz", "zellij-x86_64-unknown-linux-musl"},
		{"yazi-x86_64-unknown-linux-musl.zip", "yazi-x86_64-unknown-linux-musl"},
		{"thing.tar.xz", "thing"},
		{"jq-linux-amd64", "jq-linux-amd64"},
		{"tool-1.2.3-linux", "tool-1.2.3-linux"},
	} {
		if got := tools.AssetStem(tc.in); got != tc.want {
			t.Errorf("AssetStem(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The pin is cross-checked per platform, not per tool: a project can publish a
// checksum for one target and not another, and reporting the tool as checked
// everywhere would overstate three of them.
func TestCrossCheckedIsPerPlatform(t *testing.T) {
	e := Entry{Verified: []string{"linux_x86_64"}}
	if !e.CrossChecked(platform.Info{OS: "linux", Arch: "x86_64"}) {
		t.Error("the platform that was checked reports unchecked")
	}
	if e.CrossChecked(platform.Info{OS: "darwin", Arch: "aarch64"}) {
		t.Error("a platform that was not checked reports checked")
	}
	if (Entry{}).CrossChecked(platform.Info{OS: "linux", Arch: "x86_64"}) {
		t.Error("an entry with no verified list reports checked")
	}
}

// A slot names its project's checksum file; if that name is wrong the download
// 404s, which Relock reports as "no upstream checksum published" and carries
// on. The pin silently drops to the hash of the download. The lockfile is
// where that shows: a tool that declares checksums and records none was not
// compared with anything.
func TestEveryToolThatPublishesChecksumsHasThemRecorded(t *testing.T) {
	lock, err := LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	all, err := tools.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range all {
		if tool.Checksums == "" {
			continue
		}
		entry, ok := lock.Get(tool.Name)
		if !ok {
			t.Errorf("%s is not in bothy.lock", tool.Name)
			continue
		}
		if len(entry.Verified) == 0 {
			t.Errorf("%s names a checksums file (%q) and no platform was cross-checked; "+
				"the name is probably wrong for this release", tool.Name, tool.Checksums)
		}
	}
}

// Pinning backwards is the point of naming a tag: the latest release is
// exactly the thing you are trying not to take. So RelockAt must not ask which
// release is latest -- an APIBase that fails the test proves it never does.
func TestRelockAtTakesTheTagAndNeverAsksWhatIsLatest(t *testing.T) {
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			http.NotFound(w, r)
			return
		}
		if !strings.Contains(r.URL.Path, "/v1.0.0/") {
			t.Errorf("downloaded from %s, want the tag that was asked for", r.URL.Path)
		}
		fmt.Fprint(w, "binary")
	}))
	defer assets.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("asked the releases API for %s; the tag was given", r.URL.Path)
	}))
	defer api.Close()

	oldRelease, oldAPI := ReleaseBase, APIBase
	ReleaseBase, APIBase = assets.URL, api.URL
	defer func() { ReleaseBase, APIBase = oldRelease, oldAPI }()

	tool := tools.Tool{
		Header: slots.Header{Name: "thing"},
		Fetch: slots.Fetch{Binary: "thing", Repo: "acme/thing", Checksums: "{asset}.sha256",
			Assets: map[string]string{
				"linux_x86_64": "thing", "linux_aarch64": "thing",
				"darwin_x86_64": "thing", "darwin_aarch64": "thing",
			}},
	}
	e, err := RelockAt(tool, "v1.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Tag != "v1.0.0" || e.Version != "1.0.0" {
		t.Errorf("recorded tag %q version %q, want v1.0.0 and 1.0.0", e.Tag, e.Version)
	}
	if got, want := e.SHA(platform.Info{OS: "linux", Arch: "x86_64"}), Sum([]byte("binary")); got != want {
		t.Errorf("recorded %q, want the hash of what was served (%q)", got, want)
	}
	// The checksum file 404s, so nothing was cross-checked and nothing may
	// claim to have been.
	if len(e.Verified) != 0 {
		t.Errorf("recorded %v as cross-checked with no checksum published", e.Verified)
	}
}
