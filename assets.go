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

// Slots holds the declarative tool and provider definitions. A tool bothy can
// supply is a file in here and nothing else — see docs/adding-a-provider.md.
//
//go:embed slots
var Slots embed.FS

// lockFile pins the version and checksum of every tool bothy may install. It
// is embedded because an installed bothy has no repository to read it from,
// and the pins are part of what a given bothy release is.
//
//go:embed bothy.lock
var lockFile []byte

// Lock returns the embedded lockfile.
func Lock() ([]byte, error) { return lockFile, nil }
