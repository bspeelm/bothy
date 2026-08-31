#!/bin/sh
# Set the version in the rpm spec and add a changelog entry.
#
# A separate script because the changelog entry is multi-line, and a multi-line
# insert inside a Makefile recipe is a fight with two levels of escaping that
# nobody wins.
set -eu

version="$1"
spec="packaging/bothy.spec"
name="$(git config user.name)"
email="$(git config user.email)"
# rpm wants the real weekday; rpmbuild warns loudly when it disagrees.
date="$(LC_ALL=C date '+%a %b %d %Y')"

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

sed "s/^Version:.*/Version:        $version/" "$spec" > "$tmp"

awk -v entry="* $date $name <$email> - $version-1" \
    -v url="- See https://github.com/bspeelm/bothy/releases/tag/v$version" '
    { print }
    /^%changelog$/ { print entry; print url; print "" }
' "$tmp" > "$spec"

echo "  $spec -> $version"
