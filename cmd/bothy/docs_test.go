package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The README drifted from the code during this project's own development: it
// listed commands that had been renamed and claimed the doctor detected traps
// whose checks had deliberately been removed. Prose has no compiler, so this
// stands in for one on the two things most likely to rot.

var readmeCommand = regexp.MustCompile(`(?m)^\| ` + "`" + `bothy ?([a-z-]*)[^` + "`" + `]*` + "`")

func TestEveryCommandInTheReadmeExists(t *testing.T) {
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	var checked int
	for _, m := range readmeCommand.FindAllStringSubmatch(string(readme), -1) {
		sub := m[1]
		if sub == "" {
			continue // bare `bothy`, which is main's no-argument path
		}
		checked++
		if !strings.Contains(string(main), `case "`+sub+`"`) {
			t.Errorf("README documents `bothy %s`, which main.go does not handle", sub)
		}
	}
	if checked < 5 {
		t.Errorf("only found %d commands in the README table; the pattern has drifted", checked)
	}
}

// Every subcommand should be documented, or it may as well not exist.
func TestEveryCommandIsInTheUsage(t *testing.T) {
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(main)
	usageStart := strings.Index(body, "const usage =")
	usageEnd := strings.Index(body[usageStart:], "`\n")
	usage := body[usageStart : usageStart+usageEnd]

	// The help paths do not need their own usage line, and `lock` is a
	// maintainer command: it downloads half a gigabyte to recompute checksums,
	// and advertising it to everyone who types `bothy help` invites that.
	undocumented := map[string]bool{
		"version": true, "--version": true, "-v": true,
		"help": true, "--help": true, "-h": true,
		"lock": true,
	}

	for _, m := range regexp.MustCompile(`(?m)^\tcase "([a-z-]+)"`).FindAllStringSubmatch(body, -1) {
		cmd := m[1]
		if undocumented[cmd] {
			continue
		}
		// Whole words, not substrings. This checked strings.Contains, and
		// `bothy lock` -- which was genuinely missing from the usage text --
		// was satisfied by the word "unlocked" in the first line. The one
		// command the test existed to catch was the one it could not see.
		if !regexp.MustCompile(`\bbothy ` + cmd + `\b`).MatchString(usage) {
			t.Errorf("`bothy %s` exists but is not in the usage text", cmd)
		}
	}
}

// Version must stay a plain constant string.
//
// `-X main.Version=…` silently does nothing to a variable initialised by a
// function call — no error, no warning, the release build just reports "dev".
// An earlier attempt to fold the build-info fallback into this declaration did
// exactly that, and only comparing the output of two builds caught it.
func TestVersionStaysStampable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^var Version = "[^"]*"$`).Match(src) {
		t.Error(`Version is not a plain string literal; -X will silently stop working`)
	}
}

// A README linking to a file that is not there is worse than no link: it is
// the first thing a newcomer clicks, and this project's pitch is its
// documentation. `[PLAN.md](PLAN.md)` pointed at the repository root for
// several releases while the file lived in docs/.
func TestEveryRelativeDocLinkResolves(t *testing.T) {
	root := "../.."
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	// history/ is globbed separately: a file moved there would otherwise stop
	// being checked by the move itself.
	past, err := filepath.Glob(filepath.Join(root, "docs", "history", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, past...)
	// packaging/ documents the release channels and was outside this test
	// entirely, so its links were never checked.
	pkg, err := filepath.Glob(filepath.Join(root, "packaging", "**", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	top, err := filepath.Glob(filepath.Join(root, "packaging", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, append(pkg, top...)...)
	files = append(files, filepath.Join(root, "README.md"),
		filepath.Join(root, "CLAUDE.md"), filepath.Join(root, "NOTICE"),
		filepath.Join(root, "CONTRIBUTING.md"), filepath.Join(root, "SECURITY.md"))

	link := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			continue // NOTICE has no links; a missing optional file is not a failure
		}
		for _, m := range link.FindAllStringSubmatch(string(body), -1) {
			target := m[1]
			if strings.HasPrefix(target, "http") {
				continue
			}
			path, frag, hasFrag := strings.Cut(target, "#")
			// A bare "#anchor" points inside the file it is written in.
			at := f
			if path != "" {
				at = filepath.Join(filepath.Dir(f), path)
				if _, err := os.Stat(at); err != nil {
					t.Errorf("%s links to %q, which does not exist",
						filepath.Base(f), m[1])
					continue
				}
			}
			if hasFrag && frag != "" && !hasHeading(t, at, frag) {
				t.Errorf("%s links to %q, and %s has no such heading",
					filepath.Base(f), m[1], filepath.Base(at))
			}
		}
	}
}

// buildTagged is every shipping file the compiler picks by platform. ADR-031
// allows one only where the code cannot compile elsewhere, and requires it to
// stay a shim: the logic goes outside, behind a seam a test can replace, so
// that what CI never compiles is also what decides nothing.
//
// A list rather than a rule about names, because adding one is the decision
// the ADR asks to be taken deliberately. Tests are exempt -- they gate which
// tests run, not what ships.
var buildTagged = map[string]int{
	"internal/platform/termsize_unix.go":  30, // ioctl TIOCGWINSZ; no such constant on Windows
	"internal/platform/termsize_other.go": 12,
}

func TestPlatformSplitsStayShims(t *testing.T) {
	root := "../.."
	found := map[string]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") || strings.Contains(path, "/vendor/") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(src), "//go:build") {
			return nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
		found[rel] = true
		limit, ok := buildTagged[rel]
		if !ok {
			t.Errorf("%s is built per platform and ADR-031 does not list it.\n"+
				"Platform differences are injected at runtime unless the code cannot "+
				"compile elsewhere; if this one cannot, add it here with the reason.", rel)
			return nil
		}
		if n := bytes.Count(src, []byte("\n")); n > limit {
			t.Errorf("%s is %d lines, over its %d-line shim budget -- "+
				"CI never compiles the other side of a build tag, so logic here is "+
				"logic nothing checks. Move it out behind a seam.", rel, n, limit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range buildTagged {
		if !found[path] {
			t.Errorf("%s is listed as a platform split and no longer is; drop it", path)
		}
	}
}

// Publishing credentials are read at release time, after the tag is pushed and
// after CI has passed: CI runs goreleaser with --skip=publish, so these
// templates are never compiled and these variables are never read. Two bugs
// have reached a tag this way -- a workflow that set neither variable, and
// `envOrDefault`, which is not a function goreleaser defines.
//
// So the pairing is asserted here: every credential the release config reads
// is a plain .Env reference, and the workflow sets it.
func TestEveryReleaseCredentialIsSetByTheWorkflow(t *testing.T) {
	root := "../.."
	cfg, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	flow, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}

	// Anything but `{{ .Env.NAME }}` -- a helper, a default, a pipeline -- is
	// a template that compiles here and fails at release time.
	body := string(cfg)
	for _, line := range strings.Split(body, "\n") {
		field, value, ok := strings.Cut(line, ":")
		if !ok || !strings.Contains(value, "{{") {
			continue
		}
		field = strings.TrimSpace(field)
		if field != "token" && field != "private_key" {
			continue
		}
		if !regexp.MustCompile(`\{\{\s*\.Env\.[A-Z_]+\s*\}\}`).MatchString(value) {
			t.Errorf("%s uses a template that is only compiled at release time: %s",
				field, strings.TrimSpace(value))
		}
	}

	used := regexp.MustCompile(`\{\{\s*\.Env\.([A-Z_]+)\s*\}\}`).FindAllStringSubmatch(body, -1)
	if len(used) == 0 {
		t.Fatal("no credentials found in .goreleaser.yaml; this test is asserting nothing")
	}
	for _, m := range used {
		name := m[1]
		if !strings.Contains(string(flow), name+":") {
			t.Errorf(".goreleaser.yaml reads %s, which release.yml never sets", name)
		}
	}
}

// Homebrew removed --no-quarantine in 4.7. The README recommended it twice, on
// two separate attempts, and a real Mac rejected both. Prose may explain the
// removal; a command line may not carry the flag.
func TestNoBrewCommandOffersARemovedFlag(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("../..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.Contains(line, "brew install") {
			continue
		}
		seen = true
		if strings.Contains(line, "--no-quarantine") {
			t.Errorf("brew line offers a flag Homebrew removed in 4.7: %s",
				strings.TrimSpace(line))
		}
	}
	if !seen {
		t.Error("no brew install line in the README; this test is asserting nothing")
	}
}

// The container images live in two places that cannot import each other: the
// Go list is behind the `container` build tag, and CI reads its own copy to
// pre-pull and to assert each distro produced a subtest. They drifted once --
// CI pulled and asserted two while the test ran four, so a silently skipped
// Debian or Arch subtest passed unnoticed.
func TestCIAndTheContainerTestAgreeOnImages(t *testing.T) {
	root := "../.."
	goSrc, err := os.ReadFile(filepath.Join(root, "cmd", "bothy", "container_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}

	m := regexp.MustCompile(`var images = \[\]string\{([^}]*)\}`).FindSubmatch(goSrc)
	if m == nil {
		t.Fatal("no `var images` in container_test.go; this test is asserting nothing")
	}
	inGo := map[string]bool{}
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(string(m[1]), -1) {
		inGo[q[1]] = true
	}

	c := regexp.MustCompile(`IMAGES:\s*"([^"]*)"`).FindSubmatch(ci)
	if c == nil {
		t.Fatal("no IMAGES in ci.yml; this test is asserting nothing")
	}
	inCI := map[string]bool{}
	for _, f := range strings.Fields(string(c[1])) {
		inCI[f] = true
	}

	if len(inGo) == 0 {
		t.Fatal("parsed no images out of container_test.go")
	}
	for img := range inGo {
		if !inCI[img] {
			t.Errorf("container_test.go runs %q, which ci.yml neither pulls nor asserts", img)
		}
	}
	for img := range inCI {
		if !inGo[img] {
			t.Errorf("ci.yml expects %q, which container_test.go never runs", img)
		}
	}
}

// upgradeAdvice maps each install method the README offers to something
// describeInstall must say to someone who used it. A new row in the table
// with no entry here fails, which is the point: Homebrew was advertised as
// the first way in while `bothy upgrade` called it unrecognised.
var upgradeAdvice = map[string]string{
	"Script":   "install.sh",
	"Homebrew": "brew upgrade --cask",
	"dnf":      "dnf upgrade",
	"apt":      "apt install",
	"Go":       "go install",
	"Source":   "make install-binary",
}

func TestEveryInstallMethodIsRecognisedByUpgrade(t *testing.T) {
	root := "../.."
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	upgrade, err := os.ReadFile(filepath.Join(root, "cmd", "bothy", "upgradecmd.go"))
	if err != nil {
		t.Fatal(err)
	}

	// Scoped to the install table: "What you need first" above it also has
	// bold first cells, and lists prerequisites rather than channels.
	section := string(readme)
	start := strings.Index(section, "### Getting bothy")
	if start < 0 {
		t.Fatal("no '### Getting bothy' heading; the README shape has changed")
	}
	section = section[start:]
	if end := strings.Index(section, "\n### "); end > 0 {
		section = section[:end]
	}
	rows := regexp.MustCompile(`(?m)^\| \*\*([^*]+)\*\* \|`).FindAllStringSubmatch(section, -1)
	if len(rows) < 5 {
		t.Fatalf("found %d install rows; the table shape has changed", len(rows))
	}
	for _, r := range rows {
		method := r[1]
		want, known := upgradeAdvice[method]
		if !known {
			t.Errorf("the README offers %q and this test does not know what "+
				"`bothy upgrade` should say about it", method)
			continue
		}
		if !strings.Contains(string(upgrade), want) {
			t.Errorf("the README offers %q but upgradecmd.go never says %q", method, want)
		}
	}

	// "Six ways in" is prose next to a table; it drifts the moment a row moves.
	counts := map[int]string{5: "Five", 6: "Six", 7: "Seven", 8: "Eight"}
	if word, ok := counts[len(rows)]; ok &&
		!strings.Contains(section, word+" ways in") {
		t.Errorf("the table has %d rows; the README does not say %q ways in", len(rows), word)
	}
}

// hasHeading reports whether the markdown file has a heading whose GitHub
// anchor is frag. Anchors are derived from heading text, so renaming a
// heading breaks every deep link to it and nothing says so.
func hasHeading(t *testing.T, path, frag string) bool {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		return true // unreadable is the other assertion's problem
	}
	drop := regexp.MustCompile("[^a-z0-9 -]")
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		text := strings.TrimLeft(line, "# ")
		slug := strings.ReplaceAll(drop.ReplaceAllString(strings.ToLower(text), ""), " ", "-")
		if slug == frag {
			return true
		}
	}
	return false
}

// "No telemetry" is a claim in the README's not-list and in PLAN.md, and
// nothing checked it. These are the only hosts bothy is allowed to name:
// releases and their checksums, and the install script. Anything else in
// shipping code is either a new dependency on someone's availability or the
// thing the not-list says bothy does not do.
var allowedHosts = map[string]bool{
	"github.com":                true,
	"api.github.com":            true,
	"raw.githubusercontent.com": true,
}

func TestShippingCodeNamesNoHostButGitHub(t *testing.T) {
	root := "../.."
	host := regexp.MustCompile(`https?://([a-zA-Z0-9.-]+)`)
	found := 0
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range host.FindAllStringSubmatch(string(body), -1) {
				found++
				if !allowedHosts[m[1]] {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s reaches %q; bothy talks to GitHub and nothing else", rel, m[1])
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if found == 0 {
		t.Fatal("no host literals found at all; this test is asserting nothing")
	}
}

// What uninstall leaves is stated in seven prose sites and enforced by one
// function, and the two have disagreed five times: #142 fixed two comments,
// #143 the help text, #176 docs/PLAN.md, and the README twice. The true
// shape is in internal/install/uninstall.go -- the tree and the binary go,
// three things are named on the way out.
//
// This is a regression guard, not a proof: it bans the phrasings that have
// actually shipped rather than deriving the claim from the code.
var retiredUninstallClaims = []string{
	"removes two directories",
	"remove two directories",
	"one folder goes and nothing else does",
	"uninstall leaves nothing",
	"removes everything it wrote",
}

func TestNoDocRepeatsARetiredUninstallClaim(t *testing.T) {
	root := "../.."
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join(root, "README.md"),
		filepath.Join(root, "CONTRIBUTING.md"), filepath.Join(root, "SECURITY.md"))
	// The help text carried this one too, so the source is in scope.
	files = append(files, filepath.Join(root, "cmd", "bothy", "main.go"))

	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(body))
		for _, claim := range retiredUninstallClaims {
			if strings.Contains(lower, claim) {
				t.Errorf("%s says %q; uninstall removes the tree and the binary "+
					"and names three leftovers", filepath.Base(f), claim)
			}
		}
	}
}

// The wiki is a separate git repository, so its links into this one are
// absolute URLs -- which TestEveryRelativeDocLinkResolves skips as external.
// Its whole design is short answers deep-linking into decisions.md, and
// anchors derive from heading text, so a retitled ADR breaks every link to it
// silently.
//
// It catches a broken link, not a wrong one: an anchor pointing at the wrong
// ADR still resolves, and only a reader notices. Writing three of these by
// hand produced exactly that, and this test would not have caught it.
func TestWikiLinksIntoThisRepoResolve(t *testing.T) {
	root := "../.."
	pages, err := filepath.Glob(filepath.Join(root, "wiki", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Skip("no wiki pages yet")
	}

	blob := regexp.MustCompile(`https://github\.com/bspeelm/bothy/blob/main/([^)#\s]+)(#([^)\s]+))?`)
	checked := 0
	for _, page := range pages {
		body, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range blob.FindAllStringSubmatch(string(body), -1) {
			checked++
			target := filepath.Join(root, m[1])
			if _, err := os.Stat(target); err != nil {
				t.Errorf("%s links to %q, which is not in this repository",
					filepath.Base(page), m[1])
				continue
			}
			if m[3] != "" && !hasHeading(t, target, m[3]) {
				t.Errorf("%s links to %s#%s, and that heading does not exist",
					filepath.Base(page), m[1], m[3])
			}
		}
	}
	if checked == 0 {
		t.Error("no links from wiki/ into this repository; this test is asserting nothing")
	}
}
