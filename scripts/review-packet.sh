#!/bin/sh
# Generates a review packet skeleton, or checks that one has been answered.
#
#   scripts/review-packet.sh 0.9.0          write docs/review/0.9.0.md
#   scripts/review-packet.sh --check 0.9.0  fail unless it exists and is answered
#
# The change map is produced from the diff, never written by hand: a change the
# author's summary forgets still appears, because diffs have no instincts.
set -eu

UNANSWERED='_Unanswered._'

# The packets are the review, not a thing under review. Without this a packet
# is in the diff it maps and cannot name itself, because the map is written
# before the file exists -- and every release is blocked instead of an
# unreviewed one.
mapped() { git diff --name-only "$1" | grep -v '^docs/review/' || true; }

check=0
if [ "${1:-}" = "--check" ]; then check=1; shift; fi
ver=${1:-}
test -n "$ver" || { echo "usage: $0 [--check] <version>"; exit 2; }
packet="docs/review/$ver.md"

if [ "$check" = 1 ]; then
    test -f "$packet" || {
        echo "no $packet -- the release is blocked until the packet exists"
        echo "  write it with: scripts/review-packet.sh $ver"
        exit 1
    }
    if grep -qF "$UNANSWERED" "$packet"; then
        echo "$packet still has unanswered sections:"
        grep -nF "$UNANSWERED" "$packet" | sed 's/^/  line /'
        exit 1
    fi
    # The map is generated, so a file cannot be dropped from it by editing.
    base=$(sed -n 's/^Base: `\([^`]*\)`.*/\1/p' "$packet" | head -1)
    case "$base" in "(none)"|"") range=HEAD ;; *) range="$base..HEAD" ;; esac
    missing=0
    for f in $(mapped "$range"); do
        grep -qF "\`$f\`" "$packet" || {
            echo "$packet omits $f, which the diff since $base contains"
            missing=1
        }
    done
    test "$missing" = 0 || exit 1

    # Freshness is required only for the load-bearing surfaces this release
    # touches (framework 9.3), not the whole map -- otherwise every release
    # waits on every surface and the gate is one nobody can pass.
    stale=0
    for s in $(sed -n 's/^| *`\([^`]*\)` *|.*|.*|$/\1/p' docs/reviewed.md); do
        touched=0
        for f in $(mapped "$range"); do
            case "$f" in "$s"|"$s"/*) touched=1; break ;; esac
        done
        test "$touched" = 1 || continue
        sh scripts/ledger.sh | grep -q "^ok      $s " || {
            echo "$s changed in this release and is not read at its current commit"
            stale=1
        }
    done
    test "$stale" = 0 || {
        echo "  the ledger gates the release: see docs/reviewed.md"
        exit 1
    }

    echo "ok      $packet is answered, its map matches the diff,"
    echo "        and every load-bearing surface it touches is freshly read"
    exit 0
fi

base=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
range=${base:+$base..}HEAD
changed=$(mapped "$range")
test -n "$changed" || { echo "nothing changed since ${base:-the beginning}"; exit 1; }

# The load-bearing list is the ledger's, so the two cannot disagree.
surfaces=$(sed -n 's/^| *`\([^`]*\)` *|.*|.*|$/\1/p' docs/reviewed.md)

lb=""; rest=""
for f in $changed; do
    hit=0
    for s in $surfaces; do
        case "$f" in "$s"|"$s"/*) hit=1; break ;; esac
    done
    if [ "$hit" = 1 ]; then lb="$lb$f
"; else rest="$rest$f
"; fi
done

mkdir -p docs/review
{
    echo "# Review packet — $ver"
    echo
    echo "Base: \`${base:-(none)}\` · generated $(date -u +%Y-%m-%d) by \`scripts/review-packet.sh\`"
    echo
    echo "## What shipped"
    echo
    echo "$UNANSWERED One plain paragraph. No jargon; define any term of art in"
    echo "the sentence that introduces it."
    echo
    echo "## Change map"
    echo
    echo "Generated from \`git diff --name-only $range\`, less \`docs/review/\`."
    echo "\`--check\` recomputes it: a file this packet does not name anywhere"
    echo "fails the release."
    echo
    echo "### Load-bearing"
    echo
    if [ -n "$lb" ]; then printf '%s' "$lb" | sed 's/^/- `/;s/$/`/'; else echo "_None._"; fi
    echo
    echo "### Everything else"
    echo
    if [ -n "$rest" ]; then printf '%s' "$rest" | sed 's/^/- `/;s/$/`/'; else echo "_None._"; fi
    echo
    echo "## Review cards"
    echo
    if [ -n "$lb" ]; then
        printf '%s' "$lb" | while read -r f; do
            test -n "$f" || continue
            echo "### \`$f\`"
            echo
            echo "**What it does now.** $UNANSWERED"
            echo
            echo "**What could go wrong here.** $UNANSWERED Drafted by a model"
            echo "other than the one that wrote the code, from the spec and the"
            echo "map above. The writing agent may append, never remove."
            echo
            echo "**How to check it.** $UNANSWERED Run X, expect Y."
            echo
            echo "**Not verified by the agent.** $UNANSWERED"
            echo
        done
    else
        echo "_No load-bearing file changed._"
        echo
    fi
    echo "## Reviewer's questions"
    echo
    echo "After this ship, what *new* paths write, download, execute or delete?"
    echo
    echo "1. Writes: $UNANSWERED"
    echo "2. Bytes in, and the gate on each: $UNANSWERED"
    echo "3. Processes executed, with what environment: $UNANSWERED"
    echo "4. Deletions, and their failure modes: $UNANSWERED"
    echo "5. Claims made, and the check proving each: $UNANSWERED"
    echo
    echo "## Off-card findings"
    echo
    echo "What the review turned up that no card pointed at. \"None\" is legal"
    echo "and tracked: a run of them is a finding about the review, not the code."
    echo
    echo "$UNANSWERED"
    echo
    echo "## Witness"
    echo
    echo "Required at Low stakes and above. Name and date on receipt."
    echo
    echo "$UNANSWERED"
} > "$packet"

echo "wrote $packet"
echo "  load-bearing files changed: $(printf '%s' "$lb" | grep -c . || true)"
echo "  everything else:            $(printf '%s' "$rest" | grep -c . || true)"
