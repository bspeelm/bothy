package fetch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/probe"
	"github.com/bspeelm/bothy/internal/tools"
)

// Lockfile pins the exact version and checksum of every tool bothy may
// install, so that two machines running the same bothy get the same binaries
// and a tampered release is a hard failure rather than a silent substitution.
//
// It is regenerated deliberately by `bothy lock`, never automatically during
// an install: an installer that quietly moves its own pins is an installer
// whose output nobody can reproduce.
type Lockfile struct {
	Entries []Entry `toml:"tool"`
}

// Entry is one pinned tool.
type Entry struct {
	Name string `toml:"name"`
	// Tag is the upstream release tag, verbatim ("v0.45.1", "0.19.2",
	// "jq-1.8.2"). Version is that tag with the decoration removed, which is
	// what asset names interpolate.
	Tag     string `toml:"tag"`
	Version string `toml:"version"`
	// SHA256 is keyed by "<os>_<arch>".
	SHA256 map[string]string `toml:"sha256"`
}

// SHA returns the checksum for a platform, or "" if this entry has none.
func (e Entry) SHA(p platform.Info) string { return e.SHA256[p.OS+"_"+p.Arch] }

// LockPath is the lockfile's name at the repository root.
const LockPath = "bothy.lock"

// LoadLock reads the lockfile compiled into the binary. It is embedded rather
// than read from disk because an installed bothy has no repository to read
// from, and the pins are part of what a given bothy release *is*.
func LoadLock() (*Lockfile, error) {
	return ParseLock(bothy.Lock())
}

// ParseLock reads a lockfile from bytes.
func ParseLock(src []byte) (*Lockfile, error) {
	var l Lockfile
	if err := toml.Unmarshal(src, &l); err != nil {
		return nil, fmt.Errorf("fetch: %s: %w", LockPath, err)
	}
	return &l, nil
}

// LoadLockFile reads a lockfile from disk, for `bothy lock` to update.
func LoadLockFile(path string) (*Lockfile, error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Lockfile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	return ParseLock(src)
}

// Get returns the entry for a tool.
func (l *Lockfile) Get(name string) (Entry, bool) {
	for _, e := range l.Entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Set adds or replaces an entry.
func (l *Lockfile) Set(e Entry) {
	for i := range l.Entries {
		if l.Entries[i].Name == e.Name {
			l.Entries[i] = e
			return
		}
	}
	l.Entries = append(l.Entries, e)
}

// Save writes the lockfile, sorted so a regeneration produces a reviewable diff
// rather than a reshuffle.
func (l *Lockfile) Save(path string) error {
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].Name < l.Entries[j].Name })
	out, err := toml.Marshal(l)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	header := "# Pinned tool versions and checksums.\n" +
		"# Regenerate with 'bothy lock'; review the diff before committing.\n" +
		"# An install fetches exactly these and verifies the checksum first.\n\n"
	return os.WriteFile(path, append([]byte(header), out...), 0o644)
}

// LatestRelease asks GitHub for a repository's latest release tag.
func LatestRelease(repo string) (string, error) {
	resp, err := Client.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", fmt.Errorf("fetch: %s: %w", repo, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch: %s: HTTP %d from the releases API", repo, resp.StatusCode)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("fetch: %s: %w", repo, err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("fetch: %s: %w", repo, err)
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("fetch: %s: no tag in the latest release", repo)
	}
	return payload.TagName, nil
}

// VersionFromTag strips the decoration a project puts on its tags.
//
//	v0.45.1   -> 0.45.1     (most)
//	0.19.2    -> 0.19.2     (delta, ripgrep)
//	jq-1.8.2  -> 1.8.2      (jq)
func VersionFromTag(tag string) string {
	if i := strings.LastIndexByte(tag, '-'); i >= 0 && !isDigits(tag[:i]) {
		tag = tag[i+1:]
	}
	return strings.TrimPrefix(tag, "v")
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Platforms are the targets a lockfile records checksums for.
var Platforms = []platform.Info{
	{OS: "linux", Arch: "x86_64"},
	{OS: "linux", Arch: "aarch64"},
	{OS: "darwin", Arch: "x86_64"},
	{OS: "darwin", Arch: "aarch64"},
}

// Relock refreshes one tool's entry: resolve the latest tag, then download
// every platform's asset and record its checksum.
//
// This downloads real assets, which is slow and deliberate. A lockfile whose
// checksums were copied from a metadata endpoint rather than computed from the
// bytes bothy will actually run is a lockfile that verifies nothing.
func Relock(t tools.Tool, progress func(string)) (Entry, error) {
	tag, err := LatestRelease(t.Repo)
	if err != nil {
		return Entry{}, err
	}
	version := VersionFromTag(tag)

	e := Entry{Name: t.Name, Tag: tag, Version: version, SHA256: map[string]string{}}
	for _, p := range Platforms {
		asset, err := t.Asset(p, version)
		if err != nil {
			continue // this tool does not target that platform
		}
		url := ReleaseURL(t.Repo, tag, asset)
		if progress != nil {
			progress(fmt.Sprintf("  %s %s %s_%s", t.Name, version, p.OS, p.Arch))
		}
		body, err := Download(url)
		if err != nil {
			// A platform the project does not publish for is not an error in
			// the lockfile; it is an absent checksum, and Install refuses to
			// proceed without one.
			if progress != nil {
				progress(fmt.Sprintf("    skipped: %v", err))
			}
			continue
		}
		e.SHA256[p.OS+"_"+p.Arch] = Sum(body)
	}
	if len(e.SHA256) == 0 {
		return Entry{}, fmt.Errorf("fetch: %s %s: no asset downloaded for any platform", t.Name, tag)
	}
	return e, nil
}

// parseLockVersion is probe.ParseVersion, re-exported for tests that check a
// pinned version against the tool's own minimum.
func parseLockVersion(v string) (probe.Version, error) { return probe.ParseVersion(v) }
