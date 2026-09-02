package doctor

import "testing"

// The config check reads `ya cache clear`'s output through yaziComplaint, so
// the happy path has to be quiet in that regex's terms. It is not: the command
// announces the directory it is clearing, and the regex is deliberately loose.
// Both strings are verbatim from yazi 26.8.15.
func TestYaziComplaintSeparatesClearingFromFailing(t *testing.T) {
	clean := "Clearing cache directory: \n\"/tmp/yazi-1000\"\n"
	if yaziComplaint.Match([]byte(clean)) {
		t.Errorf("a successful `ya cache clear` reads as a config complaint: %q", clean)
	}
	broken := `Failed to parse config "/home/u/.config/yazi/yazi.toml"

Caused by:
TOML parse error at line 1, column 1
`
	if !yaziComplaint.Match([]byte(broken)) {
		t.Error("a config yazi could not parse does not read as a complaint")
	}
}
