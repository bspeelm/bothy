package fetch

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bspeelm/bothy/internal/tools"
)

// forge serves GitHub's releases endpoint. tags maps "owner/repo" to the tag
// it should report; a repo absent from the map 404s.
func forge(t *testing.T, tags map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repo := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/repos/"), "/releases/latest")
		tag, ok := tags[repo]
		if !ok {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	}))
	t.Cleanup(srv.Close)

	old := APIBase
	APIBase = srv.URL
	t.Cleanup(func() { APIBase = old })
}

func lockOf(pairs map[string]string) *Lockfile {
	l := &Lockfile{}
	for name, version := range pairs {
		l.Entries = append(l.Entries, Entry{Name: name, Version: version})
	}
	return l
}

func TestCheckOutdatedSpotsANewerRelease(t *testing.T) {
	forge(t, map[string]string{
		"zellij-org/zellij": "v0.46.0",
		"sxyazi/yazi":       "v26.8.15",
	})
	ts := []tools.Tool{
		{Name: "zellij", Repo: "zellij-org/zellij"},
		{Name: "yazi", Repo: "sxyazi/yazi"},
	}
	got := CheckOutdated(ts, lockOf(map[string]string{"zellij": "0.45.1", "yazi": "26.8.15"}))

	if len(got) != 2 {
		t.Fatalf("got %d updates, want 2", len(got))
	}
	// Sorted by name, so yazi first.
	if got[0].Name != "yazi" || got[0].Outdated() {
		t.Errorf("yazi = %+v, want up to date", got[0])
	}
	if !got[1].Outdated() {
		t.Errorf("zellij = %+v, want outdated", got[1])
	}
	if got[1].Latest != "0.46.0" || got[1].Pinned != "0.45.1" {
		t.Errorf("zellij pinned/latest = %q/%q", got[1].Pinned, got[1].Latest)
	}
}

// The failure that matters. A tool whose check could not be made must not be
// reported as current: a job that says everything is fine because GitHub was
// rate-limiting it is worse than one that says nothing.
func TestCheckOutdatedDoesNotCallAFailedCheckCurrent(t *testing.T) {
	forge(t, map[string]string{}) // every repo 404s
	ts := []tools.Tool{{Name: "zellij", Repo: "zellij-org/zellij"}}
	got := CheckOutdated(ts, lockOf(map[string]string{"zellij": "0.45.1"}))

	if len(got) != 1 {
		t.Fatalf("got %d updates", len(got))
	}
	if got[0].Reason == "" {
		t.Fatal("a failed check reported no reason")
	}
	if got[0].Outdated() {
		t.Error("a failed check was reported as outdated")
	}
	if got[0].Latest != "" {
		t.Errorf("Latest = %q from a check that failed", got[0].Latest)
	}
}

// A tool defined in slots/tools but missing from the lockfile is a gap worth
// naming, not a tool that is up to date.
func TestCheckOutdatedReportsAToolMissingFromTheLockfile(t *testing.T) {
	forge(t, map[string]string{"sharkdp/fd": "v10.5.0"})
	ts := []tools.Tool{{Name: "fd", Repo: "sharkdp/fd"}}
	got := CheckOutdated(ts, lockOf(nil))

	if len(got) != 1 || got[0].Reason == "" {
		t.Fatalf("got %+v, want a reason", got)
	}
	if got[0].Outdated() {
		t.Error("a tool with no pin was reported as outdated")
	}
}

// The tags upstream actually uses, through the same de-decoration the lockfile
// applies, so a pin and a latest are compared in the same spelling.
func TestCheckOutdatedComparesDedecoratedVersions(t *testing.T) {
	forge(t, map[string]string{"jqlang/jq": "jq-1.8.2", "BurntSushi/ripgrep": "15.2.0"})
	ts := []tools.Tool{
		{Name: "jq", Repo: "jqlang/jq"},
		{Name: "rg", Repo: "BurntSushi/ripgrep"},
	}
	got := CheckOutdated(ts, lockOf(map[string]string{"jq": "1.8.2", "rg": "15.2.0"}))
	for _, u := range got {
		if u.Outdated() {
			t.Errorf("%s reported outdated: pinned %q, latest %q", u.Name, u.Pinned, u.Latest)
		}
		if u.Reason != "" {
			t.Errorf("%s: %s", u.Name, u.Reason)
		}
	}
}

// GitHub asks for a User-Agent, and a token is what keeps a shared CI runner
// off the sixty-an-hour unauthenticated limit.
func TestLatestReleaseIdentifiesItselfAndSendsAToken(t *testing.T) {
	var ua, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua, auth = r.Header.Get("User-Agent"), r.Header.Get("Authorization")
		fmt.Fprint(w, `{"tag_name":"v1.0.0"}`)
	}))
	defer srv.Close()
	old := APIBase
	APIBase = srv.URL
	defer func() { APIBase = old }()

	t.Setenv("GITHUB_TOKEN", "secret")
	if _, err := LatestRelease("a/b"); err != nil {
		t.Fatal(err)
	}
	if ua != "bothy" {
		t.Errorf("User-Agent = %q", ua)
	}
	if auth != "Bearer secret" {
		t.Errorf("Authorization = %q", auth)
	}

	t.Setenv("GITHUB_TOKEN", "")
	if _, err := LatestRelease("a/b"); err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		t.Errorf("Authorization = %q with no token set", auth)
	}
}
