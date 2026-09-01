package config

import (
	"reflect"
	"testing"
)

// #51. Keys() is reflect-derived and Set was a hand-written switch, so the two
// drifted: `extras` was listed, suggested by Nearest when someone typed it
// almost right, and then refused by Set. Anything Keys offers, Set takes.
func TestSetTakesEveryKeyKeysOffers(t *testing.T) {
	for _, key := range Keys() {
		field, err := fieldFor(reflect.ValueOf(&Config{}).Elem(), key)
		if err != nil {
			t.Errorf("Keys lists %q but Set cannot resolve it: %v", key, err)
			continue
		}
		value := "x"
		switch field.Kind() {
		case reflect.Bool:
			value = "true"
		case reflect.Slice:
			value = "a,b"
		}
		if set := allowed[key]; len(set) > 0 {
			// A constrained key needs a value it allows; the point here is
			// reachability, not the constraint.
			value = set[0]
		}
		c := Default()
		if err := c.Set(key, value); err != nil {
			t.Errorf("Set(%q, %q) = %v", key, value, err)
		}
	}
}

// The bug that made the drift visible: extras was unsettable.
func TestExtrasIsSettable(t *testing.T) {
	c := Default()
	if err := c.Set("extras", "fzf, jq"); err != nil {
		t.Fatal(err)
	}
	if len(c.Extras) != 2 || c.Extras[0] != "fzf" || c.Extras[1] != "jq" {
		t.Errorf("Extras = %v, want [fzf jq]", c.Extras)
	}
	if err := c.Set("extras", ""); err != nil {
		t.Fatal(err)
	}
	if len(c.Extras) != 0 {
		t.Errorf("Extras = %v after clearing, want empty", c.Extras)
	}
}

// A boolean that is neither true nor false is a typo, and a typo that reads as
// false is exactly the silence config.toml stopped accepting in 0.1.5.
func TestSetRefusesAMisspeltBoolean(t *testing.T) {
	c := Default()
	if err := c.Set("workspace.watermark", "flase"); err == nil {
		t.Error("workspace.watermark accepted \"flase\" and read it as false")
	}
	for _, v := range []string{"true", "yes", "on", "1", "false", "no", "off", "0"} {
		if err := c.Set("workspace.watermark", v); err != nil {
			t.Errorf("Set(workspace.watermark, %q) = %v", v, err)
		}
	}
}
