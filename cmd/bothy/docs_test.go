package main

import (
	"bytes"
	"fmt"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/fetch"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The README drifted from the code during this project's own development: it
// listed commands that had been renamed and claimed the doctor detected traps
// whose checks had deliberately been removed. Prose has no compiler, so this
// stands in for one on the two things most likely to rot.

// Each command is a heading on the wiki page: ### `bothy attach [session]`.
var documentedCommand = regexp.MustCompile("(?m)^### `bothy ?([a-z-]*)")

// Commands are documented on the wiki page, not the README, which now names
// only the three worth typing on a first day. The page is the reference, so
// it is the thing that must not drift from main.go.
func TestEveryDocumentedCommandExists(t *testing.T) {
	readme, err := os.ReadFile("../../wiki/Commands.md")
	if err != nil {
		t.Fatal(err)
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}

	var checked int
	for _, m := range documentedCommand.FindAllStringSubmatch(string(readme), -1) {
		sub := m[1]
		if sub == "" {
			continue // bare `bothy`, which is main's no-argument path
		}
		checked++
		if !strings.Contains(string(main), `case "`+sub+`"`) {
			t.Errorf("wiki/Commands.md documents `bothy %s`, which main.go does not handle", sub)
		}
	}
	if checked < 5 {
		t.Errorf("only found %d commands on the wiki page; the pattern has drifted", checked)
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
	"internal/platform/mounted_unix.go":   34, // syscall.Stat_t.Dev; Windows has no st_dev
	"internal/platform/mounted_other.go":  8,
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
	body, err := os.ReadFile(filepath.Join("../..", "wiki", "Installing.md"))
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
		t.Error("no brew install line on the install page; this test is asserting nothing")
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
	"script":   "install.sh",
	"homebrew": "brew upgrade --cask",
	"dnf":      "dnf upgrade",
	"apt":      "apt install",
	"go":       "go install",
	"source":   "make install-binary",
}

func TestEveryInstallMethodIsRecognisedByUpgrade(t *testing.T) {
	root := "../.."
	readme, err := os.ReadFile(filepath.Join(root, "wiki", "Installing.md"))
	if err != nil {
		t.Fatal(err)
	}
	upgrade, err := os.ReadFile(filepath.Join(root, "cmd", "bothy", "upgradecmd.go"))
	if err != nil {
		t.Fatal(err)
	}

	// Scoped to the channel table: "What you need first" above it also has
	// bold first cells, and lists prerequisites rather than channels.
	section := string(readme)
	start := strings.Index(section, "## Every channel")
	if start < 0 {
		t.Fatal("no '## Every channel' heading; the install page shape has changed")
	}
	section = section[start:]
	if end := strings.Index(section, "\n## "); end > 0 {
		section = section[:end]
	}
	rows := regexp.MustCompile(`(?m)^\| \*\*([^*]+)\*\* \|`).FindAllStringSubmatch(section, -1)
	if len(rows) < 5 {
		t.Fatalf("found %d install rows; the table shape has changed", len(rows))
	}
	for _, r := range rows {
		method := strings.ToLower(r[1])
		want, known := upgradeAdvice[method]
		if !known {
			t.Errorf("the install page offers %q and this test does not know what "+
				"`bothy upgrade` should say about it", method)
			continue
		}
		if !strings.Contains(string(upgrade), want) {
			t.Errorf("the install page offers %q but upgradecmd.go never says %q", method, want)
		}
	}

	// The README counts the channels in prose while the table lives on the
	// wiki, so the two are now in different files and drift more easily.
	front, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[int]string{5: "five", 6: "six", 7: "seven", 8: "eight"}
	if word, ok := counts[len(rows)]; ok &&
		!strings.Contains(strings.ToLower(string(front)), word+" ways in") {
		t.Errorf("the install page lists %d channels; the README does not say %q ways in",
			len(rows), word)
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
	// Shipped on every release page through v0.12.0: the binary is outside the
	// tree, so removing the tree alone leaves bothy runnable.
	"removes that one directory",
	// A count of the leftovers goes stale whenever one is added -- completions
	// made it four in 0.12.0 -- and nothing here can check it.
	"names the three things",
	"the three things it leaves",
}

// proseSurfaces is every file whose prose someone relies on -- users mostly,
// maintainers for the packaging runbook -- in one place rather than appended to
// each guard that needs it. Both prose guards
// grew their file lists by hand and both had holes: one skipped _test.go, the
// other skipped the release footer, and the uninstall claim then survived in a
// third place neither read -- the rpm %description, which is what `dnf info`
// prints.
func proseSurfaces(t *testing.T) []string {
	t.Helper()
	root := "../.."
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"README.md", "CONTRIBUTING.md", "SECURITY.md",
		// The help text is prose that ships in the binary.
		filepath.Join("cmd", "bothy", "main.go"),
		// The release page footer and the deb description live here, and the
		// second survived a fix to the first.
		".goreleaser.yaml",
		// What `dnf info` and `pacman -Si` print.
		filepath.Join("packaging", "bothy.spec"),
		filepath.Join("packaging", "aur", "PKGBUILD"),
		filepath.Join("packaging", "README.md"),
		filepath.Join("packaging", "aur", "README.md"),
		// What the install script tells someone while it runs.
		filepath.Join("bootstrap", "install.sh"),
	} {
		files = append(files, filepath.Join(root, f))
	}
	wiki, err := filepath.Glob(filepath.Join(root, "wiki", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	return append(files, wiki...)
}

// flowed folds every run of whitespace to one space, so a claim that wraps
// across lines still matches. The deb description said "removes that one\n
// directory" and a substring check read straight past it, in the same file
// whose footer had just been corrected.
func flowed(body []byte) string {
	return strings.Join(strings.Fields(strings.ToLower(string(body))), " ")
}

func TestNoDocRepeatsARetiredUninstallClaim(t *testing.T) {
	for _, f := range proseSurfaces(t) {
		body, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		text := flowed(body)
		for _, claim := range retiredUninstallClaims {
			if strings.Contains(text, claim) {
				t.Errorf("%s says %q; uninstall removes the tree and the binary "+
					"and names what it leaves", filepath.Base(f), claim)
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

// A link that did not survive the shell is invisible to the check above: it
// finds what it recognises, so a target left as `$R/docs/...` is not a broken
// link, it is no link at all, and everything passes.
func TestNoWikiLinkCarriesAnUnexpandedVariable(t *testing.T) {
	pages, err := filepath.Glob(filepath.Join("../..", "wiki", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	target := regexp.MustCompile(`\]\(([^)]*)\)`)
	for _, page := range pages {
		body, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range target.FindAllStringSubmatch(string(body), -1) {
			if strings.ContainsAny(m[1], "$`") {
				t.Errorf("%s links to %q, which never expanded",
					filepath.Base(page), m[1])
			}
		}
	}
}

// Home is the wiki's index; a page it does not list is a page nobody finds,
// and a link to a page that does not exist is a dead end. Both are easy to
// leave behind when pages are added or renamed.
func TestHomeIndexesEveryWikiPage(t *testing.T) {
	dir := filepath.Join("../..", "wiki")
	pages, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	home, err := os.ReadFile(filepath.Join(dir, "Home.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Wiki-internal links are bare page names: [text](Page-Name).
	linked := map[string]bool{}
	for _, m := range regexp.MustCompile(`\]\(([A-Za-z][A-Za-z0-9-]*)\)`).
		FindAllStringSubmatch(string(home), -1) {
		linked[m[1]] = true
	}
	if len(linked) == 0 {
		t.Fatal("Home.md links to no pages; this test is asserting nothing")
	}

	exists := map[string]bool{}
	for _, p := range pages {
		exists[strings.TrimSuffix(filepath.Base(p), ".md")] = true
	}
	for name := range exists {
		if name != "Home" && !linked[name] {
			t.Errorf("wiki/%s.md exists but Home does not link it", name)
		}
	}
	for name := range linked {
		if !exists[name] {
			t.Errorf("Home links to %q, which is not a page", name)
		}
	}
}

// ci.yml lets a markdown-only change skip the jobs that build and install
// bothy. Two jobs must never be in that set: `check` runs the tests in this
// file, which read the markdown, and `no-paid-palette` scans it for colour
// values. Gating either of them would mean prose stopped being checked on
// exactly the changes that are only prose.
func TestTheJobsThatReadProseAreNotSkippedForProse(t *testing.T) {
	ci, err := os.ReadFile(filepath.Join("../..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}

	// Each job is a two-space key; its body is everything up to the next one.
	job := regexp.MustCompile(`(?m)^  ([a-z][a-z-]*):\n((?:(?:    .*)?\n)*)`)
	gated := map[string]bool{}
	for _, m := range job.FindAllStringSubmatch(string(ci), -1) {
		gated[m[1]] = strings.Contains(m[2], "needs: scope")
	}
	if len(gated) == 0 {
		t.Fatal("parsed no jobs out of ci.yml; this test is asserting nothing")
	}

	for _, name := range []string{"check", "no-paid-palette"} {
		if _, ok := gated[name]; !ok {
			t.Errorf("ci.yml has no %q job; this test is asserting nothing about it", name)
		} else if gated[name] {
			t.Errorf("the %q job is gated on scope, so a prose-only change would not run it", name)
		}
	}
	for _, name := range []string{"isolation", "container", "buildroot", "macos", "deb"} {
		if _, ok := gated[name]; !ok {
			t.Errorf("ci.yml has no %q job; this test is asserting nothing about it", name)
		} else if !gated[name] {
			t.Errorf("the %q job is not gated on scope, so prose changes still pay for it", name)
		}
	}
}

// A count of the tools bothy fetches is written in prose in several places and
// nothing moved them when the eighth was added -- the outdated workflow still
// said nine. Numbers that describe the current state have to match it.
//
// docs/decisions.md and docs/history/ are exempt: an ADR records what was true
// when the decision was made, and editing that to match today is rewriting the
// record rather than correcting it.
func TestEveryToolCountInProseMatchesTheLock(t *testing.T) {
	lock, err := fetch.LoadLock()
	if err != nil {
		t.Fatal(err)
	}
	words := map[string]int{
		"two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
		"seven": 7, "eight": 8, "nine": 9, "ten": 10,
	}
	count := regexp.MustCompile(`(?i)\b([a-z]+|\d+) tools\b`)

	root := "../.."
	var checked int
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch {
			case d.Name() == "vendor", d.Name() == ".git", d.Name() == "history":
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".yml" {
			return nil
		}
		if filepath.Base(path) == "decisions.md" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, m := range count.FindAllStringSubmatch(string(body), -1) {
			n, ok := words[strings.ToLower(m[1])]
			if !ok {
				if v, err := strconv.Atoi(m[1]); err == nil {
					n, ok = v, true
				}
			}
			if !ok {
				continue // "the tools", "these tools" and friends
			}
			checked++
			if n != len(lock.Entries) {
				t.Errorf("%s says %q; bothy.lock pins %d", rel, m[0], len(lock.Entries))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no tool count found anywhere; this test is asserting nothing")
	}
}

// The budgets are written down in three places and enforced in one. A raise
// that updated the Makefile and not the prose would leave CONTRIBUTING.md
// telling a contributor a limit that is not the limit.
func TestTheDocumentedBudgetsMatchTheMakefile(t *testing.T) {
	root := "../.."
	mk, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	value := func(name string) string {
		m := regexp.MustCompile(name + `\s*:=\s*(\d+)`).FindSubmatch(mk)
		if m == nil {
			t.Fatalf("no %s in the Makefile; this test is asserting nothing", name)
		}
		return string(m[1])
	}
	code, ratio := value("MAX_CODE_LINES"), value("MAX_COMMENT_RATIO")
	// 6500 is written "6,500" in prose.
	pretty := code[:len(code)-3] + "," + code[len(code)-3:]

	for _, f := range []string{"CONTRIBUTING.md", filepath.Join("docs", "PLAN.md")} {
		body, err := os.ReadFile(filepath.Join(root, f))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if !strings.Contains(s, pretty) && !strings.Contains(s, code) {
			t.Errorf("%s never states the code cap of %s", f, pretty)
		}
		if !strings.Contains(s, ratio+"%") {
			t.Errorf("%s never states the comment ratio of %s%%", f, ratio)
		}
	}
}

// Four files build the binary that ships -- the Makefile, goreleaser's build
// stanza and its AUR recipe, and the rpm spec -- and each was written on its
// own. v0.9.0 went out with 69 absolute paths from the CI runner baked into
// it, because none of them passed -trimpath. Adding it to one and not the
// rest would make the channels disagree, which is the drift #60 exists to
// prevent.
func TestEveryShippingBuildIsTrimmed(t *testing.T) {
	root := "../.."
	files := []string{
		"Makefile",
		".goreleaser.yaml",
		filepath.Join("packaging", "aur", "PKGBUILD"),
		filepath.Join("packaging", "bothy.spec"),
	}
	var checked int
	for _, name := range files {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(body), "\n") {
			if !strings.Contains(line, "go build") {
				continue
			}
			// The crossbuild target compiles four platforms to /dev/null to
			// prove they still compile; nothing ships out of it.
			if strings.Contains(line, "-o /dev/null") {
				continue
			}
			checked++
			if !strings.Contains(line, "-trimpath") {
				t.Errorf("%s builds without -trimpath:\n  %s", name, strings.TrimSpace(line))
			}
		}
	}
	if checked < len(files) {
		t.Fatalf("found %d shipping builds across %d files; the recipes have moved",
			checked, len(files))
	}
}

// goreleaserBlock returns one top-level block of .goreleaser.yaml.
func goreleaserBlock(t *testing.T, key string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case line == key+":":
			in = true
		case in && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#"):
			return strings.Join(out, "\n")
		case in:
			out = append(out, line)
		}
	}
	if !in {
		t.Fatalf("no %q block in .goreleaser.yaml", key)
	}
	return strings.Join(out, "\n")
}

// The release page carries its own install instructions, generated from
// .goreleaser.yaml and never from the README, so the two drift without a
// word. v0.11.2 shipped a footer still leading with the script the README had
// demoted that morning -- on the busier of the two pages.
func TestTheReleaseFooterOffersTheSameChannelsAsTheREADME(t *testing.T) {
	footer := goreleaserBlock(t, "release")
	for _, want := range []string{
		"dnf install bothy",
		"apt install ./bothy",
		"brew install --cask",
		"go install github.com/bspeelm/bothy",
	} {
		if !strings.Contains(footer, want) {
			t.Errorf("the release footer never offers %q, and the README does", want)
		}
	}
	script, pkg := strings.Index(footer, "install.sh"), strings.Index(footer, "dnf install bothy")
	if script >= 0 && pkg >= 0 && script < pkg {
		t.Error("the release footer leads with the install script; the README leads " +
			"with a package manager and the release page is read more often")
	}
}

// A commit matching no changelog group is dropped, silently. v0.11.2's
// generated notes listed one of its four changes and omitted the rpm fix the
// release existed for, because two squash titles were written in plain English
// rather than with a conventional prefix.
func TestNoCommitIsDroppedFromTheReleaseNotes(t *testing.T) {
	block := goreleaserBlock(t, "changelog")
	at := strings.Index(block, "groups:")
	if at < 0 {
		t.Fatal("no changelog groups in .goreleaser.yaml")
	}
	groups := block[at:]
	if end := strings.Index(groups, "\n  filters:"); end > 0 {
		groups = groups[:end]
	}
	for _, g := range strings.Split(groups, "- title:")[1:] {
		if !strings.Contains(g, "regexp:") {
			return
		}
	}
	t.Error("every changelog group has a regexp, so a commit matching none of them " +
		"never reaches the release notes and nothing says it was dropped")
}

// The completion scripts are a third copy of the command list, in a language
// with no compiler and no import of the first two. They rot the way the README
// rotted, and nobody notices, because a completion that is merely incomplete
// still works.
//
// So both scripts are held against the dispatch switch itself, in both
// directions: every command bothy answers to is offered, and every command
// they offer exists. `lock` is excluded by the same map and for the same
// reason as the usage text.
func completionScripts(t *testing.T) (bash, zsh string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "completions", "bothy.bash"))
	if err != nil {
		t.Fatal(err)
	}
	z, err := os.ReadFile(filepath.Join("..", "..", "completions", "_bothy"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b), string(z)
}

// dispatched is every command in main.go's switch, minus the ones deliberately
// kept out of sight. Shared with TestEveryCommandIsInTheUsage's reasoning: the
// switch is the only list that cannot lie.
func dispatched(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	hidden := map[string]bool{
		"--version": true, "-v": true, "--help": true, "-h": true, "lock": true,
	}
	var out []string
	for _, m := range regexp.MustCompile(`(?m)^\tcase "([a-z-]+)"`).FindAllStringSubmatch(string(body), -1) {
		if !hidden[m[1]] {
			out = append(out, m[1])
		}
	}
	if len(out) < 10 {
		t.Fatalf("found %d dispatched commands; the switch shape has changed", len(out))
	}
	return out
}

func TestTheCompletionsOfferEveryCommand(t *testing.T) {
	bash, zsh := completionScripts(t)

	// The bash script keeps its list in one variable and the zsh script in one
	// array of name:description pairs. Read those rather than the whole file,
	// or a command named in a comment would count as offered.
	bashList := regexp.MustCompile(`(?s)commands='([^']*)'`).FindStringSubmatch(bash)
	if bashList == nil {
		t.Fatal("no commands='...' list in bothy.bash")
	}
	bashOffers := map[string]bool{}
	for _, w := range strings.Fields(bashList[1]) {
		bashOffers[w] = true
	}
	zshOffers := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\t\t'([a-z-]+):`).FindAllStringSubmatch(zsh, -1) {
		zshOffers[m[1]] = true
	}

	for _, cmd := range dispatched(t) {
		if !bashOffers[cmd] {
			t.Errorf("`bothy %s` exists and completions/bothy.bash does not offer it", cmd)
		}
		if !zshOffers[cmd] {
			t.Errorf("`bothy %s` exists and completions/_bothy does not offer it", cmd)
		}
	}

	// And back the other way, which is the direction that catches a command
	// that was renamed rather than one that was added.
	exists := map[string]bool{}
	for _, cmd := range dispatched(t) {
		exists[cmd] = true
	}
	for name, offers := range map[string]map[string]bool{
		"completions/bothy.bash": bashOffers,
		"completions/_bothy":     zshOffers,
	} {
		for cmd := range offers {
			if !exists[cmd] {
				t.Errorf("%s offers `bothy %s`, which does not exist", name, cmd)
			}
		}
	}
}

// Flags were the one part of the command surface nothing checked. There is no
// list of them outside the flag.NewFlagSet calls, so the usage text, the wiki
// and now the completions could each disagree with the code and with each
// other in silence.
//
// Every command file declares exactly one flag set, named for its command,
// which is what makes a flag attributable to a command by reading alone.
func TestTheCompletionsOfferEveryFlag(t *testing.T) {
	bash, zsh := completionScripts(t)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	set := regexp.MustCompile(`flag\.NewFlagSet\("([a-z-]+)"`)
	decl := regexp.MustCompile(`fs\.(?:String|Bool|Int|Duration|Float64)\("([a-z-]+)"`)

	found := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := set.FindSubmatch(body)
		if name == nil {
			continue
		}
		cmd := string(name[1])
		// `lock` is not offered at all, so its flags are not either.
		if cmd == "lock" {
			continue
		}
		for _, m := range decl.FindAllSubmatch(body, -1) {
			flag := "--" + string(m[1])
			found++
			if !strings.Contains(bash, flag) {
				t.Errorf("`bothy %s %s` exists (%s) and completions/bothy.bash does not offer it", cmd, flag, f)
			}
			if !strings.Contains(zsh, flag) {
				t.Errorf("`bothy %s %s` exists (%s) and completions/_bothy does not offer it", cmd, flag, f)
			}
		}
	}
	// The parser has to keep finding flags, or this passes by seeing none.
	if found < 10 {
		t.Fatalf("found %d flags across the command files; the parser has rotted", found)
	}
}

// The command count is written in prose in two files and derived from the
// dispatch switch in a third. Adding `bothy tower` left both prose copies saying
// seventeen, and nothing failed -- the equivalent check for install channels
// already existed, so this was a gap rather than a decision.
func TestTheDocumentedCommandCountMatchesTheDispatch(t *testing.T) {
	n := len(dispatched(t))
	// `version` and `help` dispatch but are not counted as commands: the prose
	// count has always excluded them, and they are the two nobody looks up.
	n -= 2
	words := map[int]string{
		15: "fifteen", 16: "sixteen", 17: "seventeen", 18: "eighteen",
		19: "nineteen", 20: "twenty",
	}
	word, ok := words[n]
	if !ok {
		t.Fatalf("%d commands; this test does not know that number in words", n)
	}
	for _, f := range []string{"../../README.md", "../../wiki/Home.md"} {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), word) {
			t.Errorf("there are %d commands; %s does not say %q", n, filepath.Base(f), word)
		}
	}
}

// A deep link into the wiki is written as a full URL, so the relative-link test
// never sees it and its anchor goes unchecked. Anchors come from heading text,
// which means renaming a heading silently breaks every link into it -- and the
// README's links are the ones a stranger follows first.
func TestEveryWikiAnchorTheReadmeLinksToExists(t *testing.T) {
	root := "../.."
	body, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	deep := regexp.MustCompile(`https://github\.com/bspeelm/bothy/wiki/([A-Za-z-]+)#([a-z0-9-]+)`)
	found := deep.FindAllStringSubmatch(string(body), -1)
	if len(found) == 0 {
		t.Fatal("no wiki deep links in the README; this test is asserting nothing")
	}
	for _, m := range found {
		page := filepath.Join(root, "wiki", m[1]+".md")
		if _, err := os.Stat(page); err != nil {
			t.Errorf("the README links to wiki page %q, which does not exist", m[1])
			continue
		}
		if !hasHeading(t, page, m[2]) {
			t.Errorf("the README links to %s#%s; that page has no heading with that anchor", m[1], m[2])
		}
	}
}

// A vim swap file reached main and would have shipped inside the source tarball
// and the rpm. Nothing noticed, because nothing looks at what is tracked.
func TestNoEditorScratchFileIsTracked(t *testing.T) {
	out, err := exec.Command("git", "-C", "../..", "ls-files").Output()
	if err != nil {
		t.Skip("not a git checkout")
	}
	junk := regexp.MustCompile(`(?m)^.*(\.sw[a-p]|~|\.orig|\.rej|\.DS_Store)$`)
	if found := junk.FindAllString(string(out), -1); found != nil {
		t.Errorf("editor scratch files are tracked and would ship:\n  %s",
			strings.Join(found, "\n  "))
	}
}

// Local names have reached public files four times: a remote host and a sibling
// project in the tower fixtures, one path that survived the scrub of the layout
// fixtures, and a project directory used as an example session name in the wiki.
// Each was found by grepping after the fact. This looks before a release does.
//
// The words come from the machine the test runs on, so this file names nobody's
// directories. A machine that yields none -- CI, a container, a checkout under a
// generic path -- skips rather than passing quietly, because a guard that cannot
// fail should say so.
var genericPathWords = map[string]bool{
	"backup": true, "backups": true, "cache": true, "code": true,
	"config": true, "configs": true, "data": true, "desktop": true,
	"docs": true, "documents": true, "downloads": true, "home": true,
	"media": true, "project": true, "projects": true, "repos": true,
	"root": true, "scripts": true, "share": true, "source": true,
	"src": true, "state": true, "temp": true, "test": true, "tests": true,
	"tmp": true, "user": true, "users": true, "work": true, "workspace": true,
}

// localWords is what this machine is called and where it keeps its work: the
// account name, and the directory names around the checkout.
func localWords(t *testing.T) []string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Base(root)
	seen := map[string]bool{repo: true}
	var out []string
	add := func(w string) {
		l := strings.ToLower(w)
		if len(l) < 4 || genericPathWords[l] || seen[l] || strings.HasPrefix(l, ".") {
			return
		}
		seen[l] = true
		out = append(out, l)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Base(home))
	}
	// The directories the checkout sits under, and the ones beside it: a
	// sibling's name is what a pasted path or session name carries in.
	for _, part := range strings.Split(filepath.Dir(root), string(filepath.Separator)) {
		add(part)
	}
	if entries, err := os.ReadDir(filepath.Dir(root)); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				add(e.Name())
			}
		}
	}
	return out
}

func TestNoTrackedFileNamesThisMachine(t *testing.T) {
	// A checkout on a build machine sits under that machine's own directories
	// -- /home/runner/work on GitHub's -- and those words are all over the
	// workflow files legitimately. The leak starts where the names are real, so
	// this runs there: `make check` before a commit, not the CI job after it.
	if os.Getenv("CI") != "" {
		t.Skip("the words here describe the build machine, not the author's")
	}
	words := localWords(t)
	if len(words) == 0 {
		t.Skip("this machine's paths yield no distinctive words; nothing to assert")
	}
	out, err := exec.Command("git", "-C", "../..", "ls-files").Output()
	if err != nil {
		t.Skip("not a git checkout")
	}
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" || strings.HasPrefix(f, "vendor/") {
			continue
		}
		body, err := os.ReadFile(filepath.Join("../..", f))
		if err != nil {
			continue // a deleted-but-tracked path is not this test's business
		}
		lower := strings.ToLower(string(body))
		for _, w := range words {
			if strings.Contains(lower, w) {
				t.Errorf("%s carries a name from this machine's directories; "+
					"use the generic cast (api, notes, dev) instead", f)
			}
		}
	}
}

// notProse is every file the discovery below finds and proseSurfaces leaves
// out, with the reason. A file is in one list or the other, so a new one fails
// this test until someone says which -- which is the only part of #271 that
// stops a third hole appearing. Both earlier holes were file lists grown by
// hand, and neither list said what it was leaving out.
var notProse = map[string]string{
	// Generated from wiki/ by the wiki workflow; the source is in scope.
	"docs/images":            "images",
	"docs/history":           "a frozen record of what was planned, not a claim about what is",
	"CHANGELOG.md":           "generated from commits",
	".github/ISSUE_TEMPLATE": "prompts for a human, not statements about bothy",
	"CLAUDE.md":              "instructions to whoever is working here, not a claim to a user",
}

// TestEveryProseFileIsClassified fails on a prose file that no list mentions.
// Discovery is deliberately wider than proseSurfaces: the point is to notice a
// surface nobody has thought about, which is how the uninstall claim reached
// the rpm description and the wiki.
func TestEveryProseFileIsClassified(t *testing.T) {
	root := "../.."
	out, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		t.Skip("not a git checkout")
	}
	known := map[string]bool{}
	for _, f := range proseSurfaces(t) {
		rel, err := filepath.Rel(root, f)
		if err != nil {
			t.Fatal(err)
		}
		known[filepath.ToSlash(rel)] = true
	}

	excused := func(f string) bool {
		for prefix := range notProse {
			if f == prefix || strings.HasPrefix(f, prefix+"/") {
				return true
			}
		}
		return false
	}

	found := 0
	for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if f == "" || strings.HasPrefix(f, "vendor/") {
			continue
		}
		// Markdown is prose by definition. The rest are the file types that
		// have actually carried a user-facing claim: packaging metadata, the
		// install script, and the release configuration.
		if strings.HasSuffix(f, "_test.go") {
			continue // code; TestNoTrackedFileNamesThisMachine reads these
		}
		prose := strings.HasSuffix(f, ".md") ||
			strings.HasPrefix(f, "packaging/") ||
			strings.HasPrefix(f, "bootstrap/") ||
			f == ".goreleaser.yaml"
		if !prose {
			continue
		}
		found++
		if known[f] || excused(f) {
			continue
		}
		t.Errorf("%s carries prose and is in neither proseSurfaces nor notProse; "+
			"add it to the guards' list, or say why it is exempt", f)
	}
	if found == 0 {
		t.Fatal("discovery found no prose files at all; this test is asserting nothing")
	}
}

// The doctor's check count was stated in four wiki pages and enforced nowhere.
// It is now stated once and read from the registry, so adding a check fails here
// rather than making four pages wrong.
func TestTheDocumentedCheckCountMatchesTheRegistry(t *testing.T) {
	root := "../.."
	reg, err := os.ReadFile(filepath.Join(root, "internal", "doctor", "doctor.go"))
	if err != nil {
		t.Fatal(err)
	}
	n := len(regexp.MustCompile(`\{ID: "`).FindAll(reg, -1))
	if n == 0 {
		t.Fatal("no checks found in the registry; this test is asserting nothing")
	}
	page := filepath.Join(root, "wiki", "The-doctor.md")
	body, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf("%d checks", n); !strings.Contains(string(body), want) {
		t.Errorf("The-doctor.md does not say %q; the registry holds %d", want, n)
	}
	// And nowhere else states a count, or this drifts again.
	others, err := filepath.Glob(filepath.Join(root, "wiki", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	count := regexp.MustCompile(`(?i)\b(thirty|forty|\d\d)[ -]?\w* checks\b`)
	for _, f := range others {
		if filepath.Base(f) == "The-doctor.md" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := count.Find(b); m != nil {
			t.Errorf("%s states a check count (%q); the count lives in The-doctor.md alone",
				filepath.Base(f), m)
		}
	}
}

// The counts in prose that had no enforcer, which a sweep found to be two. The
// tool count comes from the lock, the command count from the dispatch, the check
// count from the registry and the budgets from the Makefile; these are the rest,
// and both were true when this was written.
//
// The box rules are read from Toolboxes.md, which owns them: Commands.md
// restated them in prose and now links instead.
func TestTheRemainingProseCountsMatchTheCode(t *testing.T) {
	root := "../.."
	// Resolve returns one Box per rule plus a fallback carrying only a reason.
	tools, err := os.ReadFile(filepath.Join(root, "internal", "install", "tools.go"))
	if err != nil {
		t.Fatal(err)
	}
	all := len(regexp.MustCompile(`return Box\{`).FindAll(tools, -1))
	fallback := len(regexp.MustCompile(`return Box\{Reason:`).FindAll(tools, -1))
	rules := all - fallback

	spelled := map[int]string{2: "two", 3: "three", 4: "four", 5: "five", 6: "six"}
	for _, c := range []struct {
		file, noun string
		want       int
	}{
		{"README.md", "slots", len(config.SlotNames())},
		{filepath.Join("wiki", "Toolboxes.md"), "rules", rules},
	} {
		word, ok := spelled[c.want]
		if !ok {
			t.Fatalf("%s: %d has no spelling here; add it", c.file, c.want)
		}
		body, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatal(err)
		}
		if want := word + " " + c.noun; !strings.Contains(flowed(body), want) {
			t.Errorf("%s does not say %q; the code has %d", c.file, want, c.want)
		}
	}
}

// A group heading in Commands.md must have a command under it. `## Answering
// from the tower` was 43 lines of prose with no `###` of its own, so the four
// entries that followed -- tools, outdated, layout, version -- became its
// children in the outline: unrelated commands filed as sub-topics of the
// tower's reply feature. The page read as disorganised because it was.
func TestEveryCommandsHeadingHasACommandUnderIt(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "wiki", "Commands.md"))
	if err != nil {
		t.Fatal(err)
	}
	group, held, groups := "", false, 0
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			if group != "" && !held {
				t.Errorf("%q has no command under it; the entries after it read as its children", group)
			}
			group, held = strings.TrimPrefix(line, "## "), false
			groups++
		case strings.HasPrefix(line, "### `bothy"):
			held = true
		}
	}
	if group != "" && !held {
		t.Errorf("%q has no command under it", group)
	}
	if groups < 3 {
		t.Fatalf("found %d group headings; the page shape has changed", groups)
	}
}
