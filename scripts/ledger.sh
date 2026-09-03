#!/bin/sh
# Reports whether each load-bearing surface in docs/reviewed.md has been read
# at its current commit. Exits 1 if any is stale, unread or missing; callers
# decide whether that blocks -- the release does, `make check` only reports.
set -eu

ledger=docs/reviewed.md
test -f "$ledger" || { echo "no $ledger"; exit 1; }

rows=$(mktemp)
trap 'rm -f "$rows"' EXIT
# Table rows are: | surface | who | commit |. Prose and the header fall out.
sed -n 's/^| *`\{0,1\}\([^ |`][^|`]*\)`\{0,1\} *| *\([^|]*\) *| *\([^|]*\) *|$/\1\t\2\t\3/p' \
    "$ledger" > "$rows"

fail=0
# Redirected, not piped: a piped `while` runs in a subshell and loses `fail`.
while IFS="$(printf '\t')" read -r surface who at; do
    surface=$(echo "$surface" | sed 's/ *$//')
    at=$(echo "$at" | tr -d ' `')
    who=$(echo "$who" | sed 's/ *$//')
    case "$surface" in surface|-*|"") continue ;; esac
    if ! test -e "$surface"; then
        echo "GONE    $surface -- in the ledger, not in the tree"
        fail=1
        continue
    fi
    now=$(git log -1 --format=%h -- "$surface")
    if [ "$at" = "-" ] || [ -z "$at" ]; then
        echo "UNREAD  $surface"
        fail=1
    elif [ "$at" != "$now" ]; then
        echo "STALE   $surface -- read at $at, now $now"
        fail=1
    else
        echo "ok      $surface -- $who at $at"
    fi
done < "$rows"

exit $fail
