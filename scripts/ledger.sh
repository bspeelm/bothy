#!/bin/sh
# Reports what each load-bearing surface in docs/reviewed.md owes: nothing, a
# read of the diff since it was last read, or a read of the whole file. Exits 1
# if anything is owed; callers decide whether that blocks -- the release does,
# `make check` only reports.
set -eu

# Past this, a read is old enough that the file has to be read whole again. A
# run of small diffs, each reviewed on its own, is how a file nobody has read
# end to end accumulates.
MAX_AGE_DAYS=30
today=$(date +%s)

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
        echo "UNREAD  $surface -- read it whole, then record the commit"
        fail=1
        continue
    fi
    if ! at_time=$(git log -1 --format=%ct "$at" 2>/dev/null); then
        echo "BADREF  $surface -- $at is not a commit in this repository"
        fail=1
        continue
    fi
    # A read is dated by the commit it was recorded at. Recording an older
    # commit dates the read older, which is the direction that errs safely.
    age=$(( (today - at_time) / 86400 ))
    if [ "$age" -gt "$MAX_AGE_DAYS" ]; then
        echo "AGED    $surface -- read $age days ago; read it whole again"
        fail=1
    elif [ "$at" != "$now" ]; then
        echo "STALE   $surface -- git diff $at..$now -- $surface"
        fail=1
    else
        echo "ok      $surface -- $who at $at"
    fi
done < "$rows"

exit $fail
