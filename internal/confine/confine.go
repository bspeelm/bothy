// Package confine runs the agent pane inside a container. The wall is the
// filesystem: the project directory and the agent's own credentials, nothing
// else from $HOME. Not a network wall -- ADR-034 says what that buys.
package confine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	bothy "github.com/bspeelm/bothy"
	"github.com/bspeelm/bothy/internal/config"
	"github.com/bspeelm/bothy/internal/platform"
	"github.com/bspeelm/bothy/internal/slots"
)

const DefaultImage = "bothy-agent"

// Dir is where the recipe lives, inside bothy's tree.
func Dir(p platform.Info) string { return filepath.Join(p.BothyDir(), "confine") }

// RecipePath is the Containerfile bothy writes.
func RecipePath(p platform.Info) string { return filepath.Join(Dir(p), "Containerfile") }

// Recipe is the shipped Containerfile.
func Recipe() (string, error) {
	b, err := bothy.Templates.ReadFile("templates/confine/Containerfile")
	if err != nil {
		return "", fmt.Errorf("confine: %w", err)
	}
	return string(b), nil
}

// WriteRecipe puts the Containerfile in bothy's tree, leaving an existing one
// alone: once written it is the user's.
func WriteRecipe(p platform.Info) (string, error) {
	path := RecipePath(p)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	body, err := Recipe()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(Dir(p), 0o755); err != nil {
		return "", fmt.Errorf("confine: %w", err)
	}
	return path, os.WriteFile(path, []byte(body), 0o644)
}

// Runtime is how bothy reaches podman. Inside a toolbox there is usually
// none, and the host's is reachable through flatpak-spawn -- the hop the
// xdg-open shim takes. Without it confinement is unavailable, never silent.
func Runtime(p platform.Info) ([]string, error) {
	if bin, err := exec.LookPath("podman"); err == nil {
		return []string{bin}, nil
	}
	if p.SharedHome {
		if fs, err := exec.LookPath("flatpak-spawn"); err == nil {
			return []string{fs, "--host", "podman"}, nil
		}
	}
	return nil, fmt.Errorf("podman is not installed")
}

// Command runs agent inside image with dir mounted.
//
// label=disable rather than a :z mount: :z relabels the user's project
// directory, which persists after uninstall. The wall is the mount set and the
// user namespace; SELinux would be a second one, not at that price.
func Command(p platform.Info, runtime []string, image, dir, agent string, creds []string) []string {
	args := append([]string{}, runtime...)
	args = append(args, "run", "--rm", "-it",
		"-v", dir+":/work:rw", "-w", "/work",
		"--userns=keep-id",
		"--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
		"--security-opt", "label=disable",
	)
	// Its own credentials, and only those: without them it cannot
	// authenticate and the wall protects nothing wanted.
	for _, c := range creds {
		args = append(args, "-v", c+":/agent/"+filepath.Base(c)+":rw")
	}
	return append(args, image, agent)
}

// Credentials are the paths to mount: what config names, else what the
// provider declares, keeping only those that exist here. Per provider rather
// than hardcoded, for the reason the detection variables are -- two lists of
// which agents exist will disagree. Empty means bothy does not know them.
func Credentials(p platform.Info, cfg []string, pr slots.Provider) []string {
	want := cfg
	if len(want) == 0 {
		want = pr.Credentials
	}
	var out []string
	for _, c := range want {
		path := config.Expand(c, p.Home)
		if _, err := os.Stat(path); err == nil {
			out = append(out, path)
		}
	}
	return out
}

// ImageBuilt reports whether the image exists locally.
func ImageBuilt(runtime []string, image string) bool {
	args := append(append([]string{}, runtime[1:]...), "image", "exists", image)
	return exec.Command(runtime[0], args...).Run() == nil
}
