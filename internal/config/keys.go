package config

import (
	"reflect"
	"strings"
)

// Keys lists every key config.toml accepts, in dotted form.
//
// Derived from the struct rather than written out, because a hand-kept list is
// one that drifts the first time a field is added and nobody remembers this
// file exists.
func Keys() []string {
	return keysOf(reflect.TypeOf(Config{}), "")
}

func keysOf(t reflect.Type, prefix string) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
		if tag == "" || tag == "-" {
			continue
		}
		name := prefix + tag
		if f.Type.Kind() == reflect.Struct {
			out = append(out, keysOf(f.Type, name+".")...)
			continue
		}
		out = append(out, name)
	}
	return out
}

// Nearest returns the valid key closest to key, or "" when nothing is close
// enough to be worth suggesting.
//
// The threshold matters more than the algorithm: suggesting "profile" for
// "borwser" is worse than suggesting nothing, because it sends someone to edit
// a line that was never the problem.
func Nearest(key string) string {
	best, bestD := "", 0
	for _, k := range Keys() {
		d := distance(key, k)
		// A third of the length, rounded down, and never more than three.
		limit := min(len(k)/3, 3)
		if d > limit {
			continue
		}
		if best == "" || d < bestD {
			best, bestD = k, d
		}
	}
	return best
}

// distance is Levenshtein, two rows rather than a full matrix.
func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
