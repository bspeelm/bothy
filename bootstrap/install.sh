#!/bin/sh
# bothy bootstrap — fetch the right binary for this machine and put it in
# ~/.local/bin. This is the only shell in the project, and it stays small enough
# to read before running (PLAN.md, ADR-001).
#
#   curl -fsSL https://raw.githubusercontent.com/bothy-dev/bothy/main/bootstrap/install.sh | sh
#
# It installs bothy itself and nothing else. Run `bothy install` afterwards to
# set up the workspace.
set -eu

REPO="bothy-dev/bothy"
VERSION="${BOTHY_VERSION:-latest}"
BINDIR="${BOTHY_BINDIR:-$HOME/.local/bin}"

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
    url="https://github.com/$REPO/releases/latest/download/bothy_${os}_${arch}.tar.gz"
else
    url="https://github.com/$REPO/releases/download/$VERSION/bothy_${os}_${arch}.tar.gz"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "bothy: downloading $os/$arch"
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$tmp/bothy.tar.gz"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "$tmp/bothy.tar.gz" "$url"
else
    echo "bothy: need curl or wget" >&2
    exit 1
fi

tar -xzf "$tmp/bothy.tar.gz" -C "$tmp"
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
echo "next: bothy install"
