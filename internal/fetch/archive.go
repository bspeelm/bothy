package fetch

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

// MaxFile caps a single extracted file, guarding against a decompression bomb
// in an archive whose checksum we trust but whose contents we have not seen.
// The two are independent guards rather than a ratio -- MaxFile is smaller
// than MaxAsset, which reads oddly because a compressed archive is normally
// smaller than its contents. Neither number is near anything real; they exist
// so a malformed or hostile archive cannot exhaust memory.
const MaxFile = 128 << 20 // 128 MiB

// Extract pulls the wanted binaries out of a downloaded asset.
//
// Release archives disagree about layout: some put the binary at the root,
// some inside a versioned directory, and jq ships a bare binary with no
// archive at all. Rather than encode each layout as data — one more thing per
// tool to get wrong — this matches on basename anywhere in the archive, which
// is true of every asset bothy fetches.
// archiveExt names the archive format an asset appears to be, or "" when it
// looks like a bare binary. Refusing by name beats guessing: a format bothy
// cannot unpack should fail at install with the reason, not at exec.
func archiveExt(name string) string {
	for _, ext := range []string{
		".tar.bz2", ".tbz2", ".tar.zst", ".tzst", ".tar.lz", ".tar.lzma",
		".tar", ".7z", ".rar", ".gz", ".bz2", ".zst", ".xz",
	} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimPrefix(ext, ".")
		}
	}
	return ""
}

func Extract(body []byte, assetName string, want []string) (map[string][]byte, error) {
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}

	switch {
	case strings.HasSuffix(assetName, ".tar.gz"), strings.HasSuffix(assetName, ".tgz"):
		return fromTarGz(body, wanted)
	case strings.HasSuffix(assetName, ".zip"):
		return fromZip(body, wanted)
	case strings.HasSuffix(assetName, ".tar.xz"), strings.HasSuffix(assetName, ".txz"):
		// The standard library has no xz decompressor and PLAN.md §13 caps
		// dependencies at go-toml. No tool bothy currently supplies ships as
		// tar.xz; helix does, which is why it is not in slots/tools yet.
		return nil, fmt.Errorf("%s is tar.xz, which bothy cannot unpack without a new dependency", assetName)
	default:
		// A bare binary, like jq's -- but only when the name does not look
		// like an archive at all. This branch used to accept anything it did
		// not recognise, so an asset shipped as .tar.bz2 or .tar.zst would
		// have had the *compressed archive* written out as the executable,
		// mode 0755. The checksum cannot catch that: the archive is exactly
		// what was pinned. It would surface as a mystery at exec time.
		if ext := archiveExt(assetName); ext != "" {
			return nil, fmt.Errorf("%s is %s, which bothy cannot unpack", assetName, ext)
		}
		if len(wanted) != 1 {
			return nil, fmt.Errorf("%s looks like a bare binary but %d were wanted", assetName, len(wanted))
		}
		for name := range wanted {
			return map[string][]byte{name: body}, nil
		}
	}
	return nil, fmt.Errorf("unreachable")
}

func fromTarGz(body []byte, wanted map[string]bool) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	found := map[string][]byte{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		name := path.Base(h.Name)
		if !wanted[name] || found[name] != nil {
			continue
		}
		content, err := readCapped(tr)
		if err != nil {
			return nil, fmt.Errorf("tar: %s: %w", h.Name, err)
		}
		found[name] = content
	}
	return found, nil
}

func fromZip(body []byte, wanted map[string]bool) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}

	found := map[string][]byte{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if !wanted[name] || found[name] != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("zip: %s: %w", f.Name, err)
		}
		content, err := readCapped(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("zip: %s: %w", f.Name, err)
		}
		found[name] = content
	}
	return found, nil
}

// readCapped reads at most MaxFile bytes and errors if there are more.
func readCapped(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxFile {
		return nil, fmt.Errorf("file exceeds %d bytes", MaxFile)
	}
	return b, nil
}
