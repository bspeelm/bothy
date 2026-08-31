// Package fetch downloads pinned release binaries and unpacks them.
//
// Everything it installs goes into bothy's own bin/ (ADR-009), and every
// download is checked against a sha256 recorded in bothy.lock before a single
// byte is written to disk. An unpinned tool is not installed: "download
// whatever is latest and run it" is not a thing this project does.
package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bothy-dev/bothy/internal/platform"
	"github.com/bothy-dev/bothy/internal/tools"
)

// Timeout is generous enough for a large binary on a poor connection and short
// enough that a hung install eventually says so.
const Timeout = 3 * time.Minute

// MaxAsset caps a download. Every asset bothy fetches is a static binary of a
// few tens of megabytes; anything vastly larger means the URL is not what we
// think it is.
const MaxAsset = 256 << 20 // 256 MiB

// Client is the HTTP client used for downloads.
var Client = &http.Client{Timeout: Timeout}

// Result describes one installed tool.
type Result struct {
	Tool     string
	Version  string
	Binaries []string
	SHA256   string
}

// Install downloads a tool at the version pinned in the lockfile, verifies it,
// and writes its binaries into destDir.
func Install(t tools.Tool, p platform.Info, lock Entry, destDir string) (*Result, error) {
	asset, err := t.Asset(p, lock.Version)
	if err != nil {
		return nil, err
	}
	want := lock.SHA(p)
	if want == "" {
		return nil, fmt.Errorf("fetch: %s %s has no checksum for %s_%s in bothy.lock\n"+
			"      run 'bothy lock' to record one", t.Name, lock.Version, p.OS, p.Arch)
	}

	url := ReleaseURL(t.Repo, lock.Tag, asset)
	body, err := Download(url)
	if err != nil {
		return nil, err
	}

	got := Sum(body)
	if got != want {
		return nil, fmt.Errorf("fetch: checksum mismatch for %s\n"+
			"      url:      %s\n"+
			"      expected: %s\n"+
			"      got:      %s\n"+
			"      the release was changed, or the download is corrupt; nothing was installed",
			t.Name, url, want, got)
	}

	found, err := Extract(body, asset, t.Binaries())
	if err != nil {
		return nil, fmt.Errorf("fetch: %s: %w", t.Name, err)
	}
	// The primary binary is not optional; a missing extra is worth failing on
	// too, because a half-installed tool is harder to diagnose than a refusal.
	for _, want := range t.Binaries() {
		if _, ok := found[want]; !ok {
			return nil, fmt.Errorf("fetch: %s not found inside %s", want, asset)
		}
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	var installed []string
	for name, content := range found {
		dest := filepath.Join(destDir, name)
		if err := writeExecutable(dest, content); err != nil {
			return nil, fmt.Errorf("fetch: %s: %w", dest, err)
		}
		installed = append(installed, dest)
	}

	return &Result{Tool: t.Name, Version: lock.Version, Binaries: installed, SHA256: got}, nil
}

// ReleaseURL builds a GitHub release download URL.
func ReleaseURL(repo, tag, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
}

// Download fetches a URL into memory. Assets are small enough that streaming to
// disk would buy nothing, and holding the bytes means the checksum is verified
// before anything is written — a corrupt download never lands on disk at all.
func Download(url string) ([]byte, error) {
	resp, err := Client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch: %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("fetch: %s\n"+
				"      404 — the asset name in slots/tools is probably wrong for this "+
				"version or platform", url)
		}
		return nil, fmt.Errorf("fetch: %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxAsset+1))
	if err != nil {
		return nil, fmt.Errorf("fetch: %s: %w", url, err)
	}
	if len(body) > MaxAsset {
		return nil, fmt.Errorf("fetch: %s is larger than %d bytes", url, MaxAsset)
	}
	return body, nil
}

// Sum is the hex sha256 of a byte slice.
func Sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// writeExecutable writes a binary atomically. The rename matters: a partially
// written binary that is already on PATH is a worse failure than no binary.
func writeExecutable(dest string, content []byte) error {
	tmp := dest + ".bothy-tmp"
	if err := os.WriteFile(tmp, content, 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
