package main

import (
	"os"
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

// TestStdoutDevNullIsNotTerminal: /dev/null is a character device, but it is
// not a terminal, so §7's no-argument command must run ls rather than open the
// board.
func TestStdoutDevNullIsNotTerminal(t *testing.T) {
	saved := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	os.Stdout = devNull
	t.Cleanup(func() {
		os.Stdout = saved
		_ = devNull.Close()
	})
	if stdoutIsTerminal() {
		t.Errorf("%s reported as a terminal", os.DevNull)
	}
}
