package main

import (
	"os"
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
		if !strings.Contains(usage, cmd) {
			t.Errorf("`bothy %s` exists but is not in the usage text", cmd)
		}
	}
}
