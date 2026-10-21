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

// lintFixture has problems on distinct paths and lines. good.md collects its
// structural unknown-key before its earlier value error, so path and line
// sorting is not source or collection order.
func lintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Good](good.md)\n2. [Gone](gone.md)\n")
	writeFile(t, root, "work/good.md", "---\nkind: nope\narea: maps\nstate: open\nbogus: x\n---\n\n# Good\n\n[missing](missing.md)\n")
	return root
}

// cleanFixture has an existing NEXT.md and no planned topics, so lint has no
// problems.
func cleanFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Good](good.md)\n")
	writeFile(t, root, "work/good.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Good\n")
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
	want := []string{
		`work/NEXT.md:4: error: Next item links to missing topic "gone"`,
		`work/NEXT.md:4: error: link to missing topic file "gone.md"`,
		`work/good.md:2: error: kind "nope" is not configured`,
		`work/good.md:5: error: unknown key "bogus"`,
		`work/good.md:10: error: link to missing topic file "missing.md"`,
	}
	if len(lines) != len(want) {
		t.Fatalf("output = %q, want %d lines", out, len(want))
	}
	for i, line := range want {
		if lines[i] != line {
			t.Errorf("line %d = %q, want %q", i, lines[i], line)
		}
	}
}

func TestLintRootDirectoryLinkExitsOne(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \".\"\nareas = [\"maps\"]\n")
	writeFile(t, root, "NEXT.md", "# Next\n")
	writeFile(t, root, "a.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\n[missing](missing.md)\n")

	out, err := runRoot(t, root, "lint")
	var code exitError
	if !errors.As(err, &code) || int(code) != 1 {
		t.Fatalf("err = %v, want exitError(1)\n%s", err, out)
	}
	if !strings.Contains(out, "a.md:9: error: link to missing topic file \"missing.md\"") {
		t.Errorf("output = %q, want a topic-link error", out)
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

func TestLintDirectoryPlanGone(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [A](a.md)\n")
	writeFile(t, root, "work/a.md", "---\nkind: bug\narea: maps\nstate: planned\nplan: docs/plans\n---\n\n# A\n")
	if err := os.MkdirAll(filepath.Join(root, "docs", "plans"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	out, err := runRoot(t, root, "lint", "--json")
	var code exitError
	if !errors.As(err, &code) || int(code) != 1 {
		t.Fatalf("err = %v, want exitError(1)\n%s", err, out)
	}
	var got struct {
		Problems []struct {
			Rule string `json:"rule"`
		} `json:"problems"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(got.Problems) != 1 || got.Problems[0].Rule != "plan-gone" {
		t.Fatalf("problems = %+v, want exactly one plan-gone", got.Problems)
	}
}

func TestLintCleanJSON(t *testing.T) {
	root := cleanFixture(t)
	out, err := runRoot(t, root, "lint", "--json")
	if err != nil {
		t.Fatalf("lint: %v\n%s", err, out)
	}
	var got struct {
		Problems json.RawMessage `json:"problems"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if string(got.Problems) != "[]" {
		t.Errorf("problems = %s, want []", got.Problems)
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
	want := []struct {
		Path string
		Line int
		Rule string
	}{
		{"work/NEXT.md", 4, "next-item"},
		{"work/NEXT.md", 4, "topic-link"},
		{"work/good.md", 2, "kind"},
		{"work/good.md", 5, "unknown-key"},
		{"work/good.md", 10, "topic-link"},
	}
	if len(got.Problems) != len(want) {
		t.Fatalf("problems = %+v, want %d", got.Problems, len(want))
	}
	for i, w := range want {
		p := got.Problems[i]
		if p.Path != w.Path || p.Line != w.Line || p.Rule != w.Rule {
			t.Errorf("problems[%d] = %+v, want path %q line %d rule %q", i, p, w.Path, w.Line, w.Rule)
		}
		if p.Severity != "error" {
			t.Errorf("problems[%d].severity = %q, want error", i, p.Severity)
		}
	}
}
