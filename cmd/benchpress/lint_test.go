package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runRoot runs the root command with cwd set to root.
func runRoot(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

// lintFixture has one missing Next item, one missing topic link and one bad
// topic, on distinct paths and lines.
func lintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Good](good.md)\n2. [Gone](gone.md)\n")
	writeFile(t, root, "work/good.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Good\n\n[missing](missing.md)\n")
	return root
}

// warnFixture has no NEXT.md, so lint has one warning and exits 0.
func warnFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/good.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Good\n")
	return root
}

func TestLintTextOrdersAndExitsOne(t *testing.T) {
	root := lintFixture(t)
	out, err := runRoot(t, root, "lint")
	var code exitError
	if !errors.As(err, &code) || int(code) != 1 {
		t.Fatalf("err = %v, want exitError(1)\n%s", err, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("output = %q, want 3 lines", out)
	}
	want := []string{
		"work/NEXT.md:4: error:",
		"work/NEXT.md:4: error:",
		"work/good.md:9: error:",
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d = %q, want prefix %q", i, lines[i], prefix)
		}
	}
}

func TestLintWarningExitsZero(t *testing.T) {
	root := warnFixture(t)
	out, err := runRoot(t, root, "lint")
	if err != nil {
		t.Fatalf("lint: %v\n%s", err, out)
	}
	if !strings.Contains(out, "work/NEXT.md:1: warning:") {
		t.Errorf("output = %q, want a warning", out)
	}
}

func TestLintJSON(t *testing.T) {
	root := lintFixture(t)
	out, err := runRoot(t, root, "lint", "--json")
	var code exitError
	if !errors.As(err, &code) || int(code) != 1 {
		t.Fatalf("err = %v, want exitError(1)\n%s", err, out)
	}
	var got struct {
		Problems []struct {
			Path     string `json:"path"`
			Line     int    `json:"line"`
			Severity string `json:"severity"`
			Rule     string `json:"rule"`
			Message  string `json:"message"`
		} `json:"problems"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(got.Problems) != 3 {
		t.Fatalf("problems = %+v, want 3", got.Problems)
	}
	if got.Problems[0].Rule != "next-item" || got.Problems[0].Severity != "error" {
		t.Errorf("problems[0] = %+v", got.Problems[0])
	}
	if got.Problems[2].Path != "work/good.md" || got.Problems[2].Rule != "topic-link" {
		t.Errorf("problems[2] = %+v", got.Problems[2])
	}
}
