#!/bin/sh
# bothy bootstrap — fetch the right binary for this machine and put it in
# ~/.local/bin. This is the only shell in the project, and it stays small enough
# to read before running (PLAN.md, ADR-001).
#
#   curl -fsSL https://raw.githubusercontent.com/bspeelm/bothy/main/bootstrap/install.sh | sh
#
# It installs bothy itself and nothing else. Run `bothy install` afterwards to
# set up the workspace.
set -eu

REPO="${BOTHY_REPO:-bspeelm/bothy}"
BASE="${BOTHY_BASE_URL:-https://github.com/$REPO/releases}"
VERSION="${BOTHY_VERSION:-latest}"
BINDIR="${BOTHY_BINDIR:-$HOME/.local/bin}"

for a in "$@"; do
    case "$a" in --verify) BOTHY_VERIFY=1 ;; esac
done

case "$(uname -s)" in
    Linux)  os=linux ;;
    Darwin) os=darwin ;;
    *) echo "bothy: unsupported system $(uname -s)" >&2
       echo "       native Windows is not supported; use WSL2." >&2
       exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64)  arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "bothy: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

if [ "$VERSION" = latest ]; then
    base="$BASE/latest/download"
else
    base="$BASE/download/$VERSION"
fi
archive="bothy_${os}_${arch}.tar.gz"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# fetch <url> <destination>. Either curl or wget; both are told to fail loudly
# on an HTTP error rather than writing the error page to the file.
fetch() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$1" -o "$2"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$2" "$1"
    else
        echo "bothy: need curl or wget" >&2
        exit 1
    fi
}

echo "bothy: downloading $os/$arch"
fetch "$base/$archive" "$tmp/$archive"

# Verify against checksums.txt from the same release. This catches corruption
# and truncation, not a compromised release; it is a checksum, not a signature.
if command -v sha256sum >/dev/null 2>&1; then
    sha_cmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
    sha_cmd="shasum -a 256"
fi

if [ -n "${sha_cmd:-}" ] && fetch "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
    want="$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1 }' "$tmp/checksums.txt")"
    if [ -z "$want" ]; then
        echo "bothy: $archive is not listed in checksums.txt" >&2
        exit 1
    fi
    got="$($sha_cmd "$tmp/$archive" | cut -d" " -f1)"
    if [ "$want" != "$got" ]; then
        echo "bothy: checksum mismatch for $archive" >&2
        echo "       expected $want" >&2
        echo "       got      $got" >&2
        exit 1
    fi
    echo "bothy: checksum verified"
else
    # Old releases predate checksums.txt, and a machine may have no sha256
    # tool at all. Say so rather than implying a check that did not happen.
    echo "bothy: no checksum available; skipping verification" >&2
fi

# Provenance, on request. The checksum above proves the bytes match what the
# release published; it cannot prove who published them, because whoever could
# swap one could swap the other. Every release artifact is signed in the
# workflow that built it, with a Sigstore certificate this repository cannot
# mint -- ADR-030 says what each level does and does not prove.
#
# Off by default because the check needs the gh CLI, and an installer that
# fails on a machine merely because gh is there is worse than one that says
# what it did not check. The bundle is a release asset so that verifying needs
# no GitHub account: asking the API for it would.
if [ -n "${BOTHY_VERIFY:-}" ]; then
    if ! command -v gh >/dev/null 2>&1; then
        echo "bothy: asked to verify provenance, but the gh CLI is not installed" >&2
        echo "       install it, or drop --verify to install with the checksum alone" >&2
        exit 1
    fi
    if ! fetch "$base/attestation.jsonl" "$tmp/attestation.jsonl" 2>/dev/null; then
        echo "bothy: asked to verify provenance, but this release publishes none" >&2
        echo "       releases before v0.4.0 predate signing" >&2
        exit 1
    fi
    if ! gh attestation verify "$tmp/$archive" --repo "$REPO" \
            --bundle "$tmp/attestation.jsonl" >/dev/null 2>&1; then
        echo "bothy: provenance verification FAILED for $archive" >&2
        echo "       these bytes do not carry a signature from $REPO's release workflow" >&2
        exit 1
    fi
    echo "bothy: provenance verified -- built by $REPO in GitHub Actions"
else
    echo "bothy: run with --verify to check provenance too (needs the gh CLI)"
fi

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$BINDIR"
install -m 755 "$tmp/bothy" "$BINDIR/bothy"

echo "bothy: installed to $BINDIR/bothy"

# ~/.local/bin missing from PATH is the single most common reason a fresh
# install appears to do nothing, so say so now rather than letting the next
# command fail with "not found".
case ":$PATH:" in
    *":$BINDIR:"*) ;;
    *) echo
       echo "bothy: $BINDIR is not on your PATH. Add it:"
       echo "       export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
esac

echo
echo "next: bothy"
