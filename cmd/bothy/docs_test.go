package main

import (
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

	// Aliases and the help paths do not need their own usage line.
	undocumented := map[string]bool{
		"dev": true, "version": true, "--version": true, "-v": true,
		"help": true, "--help": true, "-h": true,
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
	files = append(files, filepath.Join(root, "README.md"),
		filepath.Join(root, "CLAUDE.md"), filepath.Join(root, "NOTICE"))

	link := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			continue // NOTICE has no links; a missing optional file is not a failure
		}
		for _, m := range link.FindAllStringSubmatch(string(body), -1) {
			target := m[1]
			// External links and pure anchors are somebody else's problem.
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "#") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(f), target)); err != nil {
				t.Errorf("%s links to %q, which does not exist",
					filepath.Base(f), m[1])
			}
		}
	}
}
