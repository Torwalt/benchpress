package main

import (
	"strings"
	"testing"
)

// TestRootNoArgsListsWhenNotATerminal: under a test, stdout is a pipe, so the
// root command with no arguments runs ls as §7 says.
func TestRootNoArgsListsWhenNotATerminal(t *testing.T) {
	root := fixture(t)
	out, err := runRoot(t, root)
	if err != nil {
		t.Fatalf("benchpress: %v\n%s", err, out)
	}
	for _, want := range []string{"Next", "raised-ground", "drafts-strand"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
