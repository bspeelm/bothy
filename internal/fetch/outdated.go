package fetch

import (
	"sort"

	"github.com/bspeelm/bothy/internal/tools"
)

// Update is what one tool's pin looks like against its latest release. Reason
// is set when the check could not be made, deliberately not folded into "up to
// date": a job reporting everything current because GitHub was rate-limiting
// it is worse than one reporting nothing.
type Update struct {
	Name   string `json:"name"`
	Repo   string `json:"repo"`
	Pinned string `json:"pinned"`
	Latest string `json:"latest,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Outdated reports whether Latest is both known and different from Pinned.
func (u Update) Outdated() bool {
	return u.Reason == "" && u.Latest != "" && u.Latest != u.Pinned
}

// CheckOutdated asks each tool's forge for its latest release and compares it
// with the lockfile pin. One API call per tool and no asset bytes; Relock
// downloads every asset because a checksum must come from the real bytes.
func CheckOutdated(ts []tools.Tool, lock *Lockfile) []Update {
	out := make([]Update, 0, len(ts))
	for _, t := range ts {
		u := Update{Name: t.Name, Repo: t.Repo}
		if e, ok := lock.Get(t.Name); ok {
			u.Pinned = e.Version
		} else {
			u.Reason = "not in the lockfile"
			out = append(out, u)
			continue
		}
		tag, err := LatestRelease(t.Repo)
		if err != nil {
			u.Reason = err.Error()
		} else {
			u.Latest = VersionFromTag(tag)
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
