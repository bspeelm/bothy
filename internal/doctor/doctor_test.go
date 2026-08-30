package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bothy-dev/bothy/internal/config"
	"github.com/bothy-dev/bothy/internal/platform"
)

// envWithConfigDir builds a doctor Env pointed at a scratch config directory.
func envWithConfigDir(t *testing.T) (Env, string) {
	t.Helper()
	dir := t.TempDir()
	return Env{
		Platform: platform.Info{
			ConfigDir: dir,
			Home:      dir,
			LocalBin:  filepath.Join(dir, ".local", "bin"),
		},
		Config: config.Default(),
	}, dir
}

func writeYazi(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "yazi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "yazi", "yazi.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A config that mentions the rename in a comment is correct. bothy's own
// generated yazi.toml does exactly that, so a naive substring match fails every
// install it performs — which is precisely what happened the first time this
// check ran against a real machine.
func TestYaziKeyCheckIgnoresComments(t *testing.T) {
	env, dir := envWithConfigDir(t)
	writeYazi(t, dir, `# Yazi 26.x — the section is [mgr]; it was renamed from [manager] in 25.4.
[mgr]
ratio = [2, 2, 4]
`)
	if got := checkYaziConfigKeys(env); got.Severity != Pass {
		t.Errorf("severity = %s (%s: %s); a comment mentioning [manager] is not a stale key",
			got.Severity, got.Summary, got.Detail)
	}
}

func TestYaziKeyCheckCatchesRealStaleTable(t *testing.T) {
	env, dir := envWithConfigDir(t)
	writeYazi(t, dir, "[manager]\nratio = [1, 4, 3]\n")
	got := checkYaziConfigKeys(env)
	if got.Severity != Fail {
		t.Errorf("severity = %s, want fail for a real [manager] table", got.Severity)
	}
	if got.Fix == "" {
		t.Error("a failing check must carry a fix line")
	}
}

// 26.x renamed `name` to `url` in filetype rules. Same anchoring rule.
func TestYaziKeyCheckCatchesStaleFiletypeRule(t *testing.T) {
	env, dir := envWithConfigDir(t)
	writeYazi(t, dir, "[mgr]\n[filetype]\nrules = [\n  name = \"*/\"\n]\n")
	if got := checkYaziConfigKeys(env); got.Severity != Fail {
		t.Errorf("severity = %s, want fail for a `name =` filetype rule", got.Severity)
	}
}

func TestYaziKeyCheckIgnoresUrlForm(t *testing.T) {
	env, dir := envWithConfigDir(t)
	writeYazi(t, dir, "[mgr]\n[filetype]\nrules = [\n  { url = \"*/\", fg = \"#BD93F9\" },\n]\n")
	if got := checkYaziConfigKeys(env); got.Severity != Pass {
		t.Errorf("severity = %s (%s), want pass for the current `url =` form", got.Severity, got.Detail)
	}
}

// A file that looks like a Ghostty config but is not named `config` is ignored
// by Ghostty without a word. That silence is the whole reason for the check.
func TestGhosttyNearMissFilenameIsReported(t *testing.T) {
	env, dir := envWithConfigDir(t)
	gh := filepath.Join(dir, "ghostty")
	if err := os.MkdirAll(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config", "config.ghostty"} {
		if err := os.WriteFile(filepath.Join(gh, name), []byte("theme = dracula\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := checkGhosttyConfigName(env)
	if got.Severity != Warn {
		t.Errorf("severity = %s, want warn", got.Severity)
	}
	if got.Detail == "" || got.Fix == "" {
		t.Error("the report should name the ignored file and say what to do")
	}
}

func TestGhosttyCorrectFilenamePasses(t *testing.T) {
	env, dir := envWithConfigDir(t)
	gh := filepath.Join(dir, "ghostty")
	if err := os.MkdirAll(gh, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gh, "config"), []byte("theme = dracula\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := checkGhosttyConfigName(env); got.Severity != Pass {
		t.Errorf("severity = %s (%s)", got.Severity, got.Detail)
	}
}

// The shim forwards to the host, and home is shared with the host, so without
// its guard the host would exec itself forever. An unguarded shim is a failure,
// not a warning.
func TestXdgOpenShimWithoutGuardFails(t *testing.T) {
	env, dir := envWithConfigDir(t)
	bin := filepath.Join(dir, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nexec flatpak-spawn --host xdg-open \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "xdg-open"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	got := checkXdgOpenShimGuard(env)
	if got.Severity != Fail {
		t.Errorf("severity = %s, want fail for an unguarded shim", got.Severity)
	}
}

func TestXdgOpenShimWithGuardPasses(t *testing.T) {
	env, dir := envWithConfigDir(t)
	bin := filepath.Join(dir, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nif [ -f /run/.containerenv ]; then exec flatpak-spawn --host xdg-open \"$@\"; fi\nexec /usr/bin/xdg-open \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "xdg-open"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := checkXdgOpenShimGuard(env); got.Severity != Pass {
		t.Errorf("severity = %s (%s)", got.Severity, got.Detail)
	}
}

// A shim someone else put there is not bothy's to judge.
func TestXdgOpenForeignShimIsSkipped(t *testing.T) {
	env, dir := envWithConfigDir(t)
	bin := filepath.Join(dir, ".local", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "xdg-open"), []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := checkXdgOpenShimGuard(env); got.Severity != Skip {
		t.Errorf("severity = %s, want skip for a shim bothy did not write", got.Severity)
	}
}

// Every check must produce a fix line when it fails. A diagnosis without a fix
// is just a nicer error message, and PLAN.md makes the fix part of the contract.
func TestEveryFailureCarriesAFix(t *testing.T) {
	env, _ := envWithConfigDir(t)
	// This env is an empty directory, so many checks will fail — which is
	// exactly the population we want to inspect.
	for _, r := range Run(env).Results {
		if r.Severity == Fail && r.Fix == "" {
			t.Errorf("check %q failed without a fix line", r.ID)
		}
	}
}

func TestReportFailedAndCounts(t *testing.T) {
	r := Report{Results: []Result{
		{Severity: Pass}, {Severity: Warn}, {Severity: Skip},
	}}
	if r.Failed() {
		t.Error("Failed() = true with no failures")
	}
	p, w, f, s := r.Counts()
	if p != 1 || w != 1 || f != 0 || s != 1 {
		t.Errorf("Counts() = %d,%d,%d,%d", p, w, f, s)
	}

	r.Results = append(r.Results, Result{Severity: Fail})
	if !r.Failed() {
		t.Error("Failed() = false with a failure present")
	}
}

// Check IDs are the stable handle for --json consumers and for referring to a
// check in an issue, so duplicates would be a real problem.
func TestCheckIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Checks() {
		if seen[c.ID] {
			t.Errorf("duplicate check ID %q", c.ID)
		}
		seen[c.ID] = true
	}
}
