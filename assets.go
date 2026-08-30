// Package bothy carries the data files compiled into the binary.
//
// Templates and profiles are embedded rather than installed alongside the
// binary so that `bothy` is genuinely one file: there is no data directory to
// keep in sync, and no way for a partially-copied install to produce a
// partially-configured workspace.
//
// This lives at the repository root because go:embed can only reach downwards
// from the package that declares it.
package bothy

import "embed"

// Templates holds every config template, keyed by its path under templates/.
//
//go:embed all:templates
var Templates embed.FS

// Profiles holds the shipped layout profiles.
//
//go:embed profiles
var Profiles embed.FS
