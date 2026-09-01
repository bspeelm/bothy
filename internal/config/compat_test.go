package config

import (
	"slices"
	"testing"
)

// v0.3.0 changed workspace.watermark from a boolean to a path and shipped.
// Save writes the whole struct, so every config bothy had ever written carried
// `watermark = false` -- and go-toml refuses a boolean for a string field,
// which made Load return an error and every command fail. Not install: every
// command, including the `config` that would have fixed it.
func TestAConfigFromBeforeTheRenameStillLoads(t *testing.T) {
	p := writeConfig(t, `
profile = 'minimal'

[workspace]
watermark = false
pane_frames = 'none'
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("a config written by an older bothy refused to load: %v", err)
	}
	if cfg.Profile != "minimal" {
		t.Errorf("profile = %q, want the value from the file", cfg.Profile)
	}
	// The key after the retired one has to survive, or the recovery has
	// swapped a loud failure for a silent one.
	if cfg.Workspace.PaneFrames != "none" {
		t.Errorf("pane_frames = %q; a key after the retired one was lost", cfg.Workspace.PaneFrames)
	}
	if !slices.Contains(cfg.Unknown, "workspace.watermark") {
		t.Errorf("Unknown = %v, want the retired key named so the user hears about it", cfg.Unknown)
	}
}

// The general case, which is what stops this recurring: a key whose type
// changed is the same problem as one whose name did, and gets the same answer.
func TestATypeThatChangedIsAWarningNotAnError(t *testing.T) {
	p := writeConfig(t, `
profile = 'minimal'

[workspace]
background_image = false
pane_frames = 'none'

[editor]
provide_config = true
`)
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("a key with the wrong type refused to load the whole file: %v", err)
	}
	if cfg.Profile != "minimal" || cfg.Workspace.PaneFrames != "none" {
		t.Errorf("keys around the bad one were lost: profile=%q pane_frames=%q",
			cfg.Profile, cfg.Workspace.PaneFrames)
	}
	// Including one in a later table, which is where a decoder that stops at
	// the error would have quietly left the default.
	if !cfg.Editor.ProvideConfig {
		t.Error("a key in a later table was lost, which is the silent failure this avoids")
	}
	if !slices.Contains(cfg.Unreadable, "workspace.background_image") {
		t.Errorf("Unreadable = %v, want the key named", cfg.Unreadable)
	}
}

// Malformed TOML is still an error. Tolerating a key bothy cannot read is not
// the same as tolerating a file that is not TOML.
func TestBrokenTOMLIsStillAnError(t *testing.T) {
	p := writeConfig(t, "profile = \nnot toml at all [[[")
	if _, err := Load(p); err == nil {
		t.Error("a file that is not TOML loaded without complaint")
	}
}
