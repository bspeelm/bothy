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
// It is an independent guard rather than a ratio against MaxAsset; neither
// number is near anything real, they exist so a malformed or hostile archive
// cannot exhaust memory.
const MaxFile = 128 << 20 // 128 MiB

// safeEntry refuses an archive entry that tries to escape where it is put.
// Nothing is written to a path the archive chose -- the bytes go into a map
// keyed by the name asked for -- so a traversing entry is already harmless. It
// is refused anyway: such an asset is not a release bothy should unpack.
func safeEntry(name string) error {
	clean := path.Clean(filepath.ToSlash(name))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("entry %q escapes the archive", name)
	}
	return nil
}

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

// Extract pulls the wanted binaries out of a downloaded asset. Release
// archives disagree about layout -- root, a versioned directory, or jq's bare
// binary -- so this matches on basename anywhere in the archive rather than
// encoding each layout as one more thing per tool to get wrong.
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
		// like an archive at all. Accepting anything unrecognised would write
		// a compressed archive out as the executable, mode 0755, and the
		// checksum cannot catch that: the archive is exactly what was pinned.
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
		if err := safeEntry(h.Name); err != nil {
			return nil, fmt.Errorf("tar: %w", err)
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
		if err := safeEntry(f.Name); err != nil {
			return nil, fmt.Errorf("zip: %w", err)
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
