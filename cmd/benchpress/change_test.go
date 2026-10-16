package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// changeFixture is a clean workbench: one topic in Next, no lint problems.
func changeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml",
		"dir = \"work\"\nkinds = [\"bug\", \"change\", \"design\", \"idea\"]\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Taxes](taxes.md)\n")
	writeFile(t, root, "work/taxes.md", "---\nkind: change\narea: economy\nstate: open\n---\n\n# Taxes\n")
	return root
}

func readAt(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestNewCmd(t *testing.T) {
	root := changeFixture(t)
	out, err := runRoot(t, root, "new", "A graphics preset", "--kind", "idea", "--area", "economy", "--next")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(out); got != "work/a-graphics-preset.md" {
		t.Errorf("output = %q, want work/a-graphics-preset.md", got)
	}
	body := readAt(t, root, "work/a-graphics-preset.md")
	if !strings.Contains(body, "state: open\n") || !strings.Contains(body, "# A graphics preset\n") {
		t.Errorf("topic body =\n%s", body)
	}
	if next := readAt(t, root, "work/NEXT.md"); !strings.Contains(next, "[A graphics preset](a-graphics-preset.md)") {
		t.Errorf("NEXT.md =\n%s", next)
	}
}

func TestNewCmdFailures(t *testing.T) {
	root := changeFixture(t)
	cases := [][]string{
		{"new", "Taxes", "--kind", "change", "--area", "economy"},
		{"new", "A new one", "--kind", "nope", "--area", "economy"},
		{"new", "A new one", "--kind", "bug", "--area", "nope"},
		{"new", "No flags"},
	}
	for _, args := range cases {
		if out, err := runRoot(t, root, args...); err == nil {
			t.Errorf("%v succeeded, want error\n%s", args, out)
		}
	}
}

func TestStateCmd(t *testing.T) {
	root := changeFixture(t)
	writeFile(t, root, "docs/plans/P.md", "plan\n")

	if out, err := runRoot(t, root, "state", "taxes", "planned", "--plan", "docs/plans/P.md"); err != nil {
		t.Fatalf("state planned: %v\n%s", err, out)
	}
	body := readAt(t, root, "work/taxes.md")
	for _, want := range []string{"state: planned\n", "plan: docs/plans/P.md\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("planned body missing %q:\n%s", want, body)
		}
	}

	if out, err := runRoot(t, root, "state", "taxes", "open"); err != nil {
		t.Fatalf("state open: %v\n%s", err, out)
	}
	if body := readAt(t, root, "work/taxes.md"); strings.Contains(body, "plan:") {
		t.Errorf("open body still has a plan:\n%s", body)
	}

	if out, err := runRoot(t, root, "state", "taxes", "planned"); err == nil {
		t.Errorf("state planned without --plan succeeded, want error\n%s", out)
	}
	if out, err := runRoot(t, root, "state", "nope", "open"); err == nil {
		t.Errorf("state on unknown slug succeeded, want error\n%s", out)
	}
}

func TestNextCmd(t *testing.T) {
	root := changeFixture(t)
	writeFile(t, root, "work/victory.md", "---\nkind: design\narea: maps\nstate: open\n---\n\n# Victory\n")

	if out, err := runRoot(t, root, "next", "add", "victory"); err != nil {
		t.Fatalf("next add: %v\n%s", err, out)
	}
	if next := readAt(t, root, "work/NEXT.md"); !strings.Contains(next, "2. [Victory](victory.md)") {
		t.Errorf("after add:\n%s", next)
	}

	if out, err := runRoot(t, root, "next", "mv", "victory", "1"); err != nil {
		t.Fatalf("next mv: %v\n%s", err, out)
	}
	if next := readAt(t, root, "work/NEXT.md"); !strings.HasPrefix(next, "# Next\n\n1. [Victory](victory.md)") {
		t.Errorf("after mv:\n%s", next)
	}

	if out, err := runRoot(t, root, "next", "rm", "victory"); err != nil {
		t.Fatalf("next rm: %v\n%s", err, out)
	}
	if next := readAt(t, root, "work/NEXT.md"); strings.Contains(next, "victory") {
		t.Errorf("after rm:\n%s", next)
	}

	if out, err := runRoot(t, root, "next", "mv", "taxes", "9"); err == nil {
		t.Errorf("next mv out of range succeeded, want error\n%s", out)
	}
	if out, err := runRoot(t, root, "next", "add", "nope"); err == nil {
		t.Errorf("next add unknown slug succeeded, want error\n%s", out)
	}
}

func TestRmCmd(t *testing.T) {
	root := changeFixture(t)
	writeFile(t, root, "work/linker.md",
		"---\nkind: bug\narea: maps\nstate: open\n---\n\n# Linker\n\nSee [Taxes](taxes.md).\n")

	out, err := runRoot(t, root, "rm", "taxes")
	if err != nil {
		t.Fatalf("rm: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(out); got != "work/linker.md" {
		t.Errorf("output = %q, want work/linker.md", got)
	}
	if _, err := os.Stat(filepath.Join(root, "work/taxes.md")); !os.IsNotExist(err) {
		t.Errorf("taxes.md still exists (err=%v)", err)
	}
	if next := readAt(t, root, "work/NEXT.md"); strings.Contains(next, "taxes") {
		t.Errorf("NEXT.md still names taxes:\n%s", next)
	}
	if out, err := runRoot(t, root, "rm", "nope"); err == nil {
		t.Errorf("rm unknown slug succeeded, want error\n%s", out)
	}
}

// TestLintCleanAfterChanges runs every operation and checks that lint reports
// nothing new afterwards.
func TestLintCleanAfterChanges(t *testing.T) {
	root := changeFixture(t)
	steps := [][]string{
		{"new", "A graphics preset", "--kind", "idea", "--area", "economy", "--next"},
		{"state", "a-graphics-preset", "settled"},
		{"next", "rm", "taxes"},
		{"next", "add", "taxes", "--at", "1"},
		{"next", "mv", "taxes", "1"},
		{"rm", "a-graphics-preset"},
	}
	for _, args := range steps {
		if out, err := runRoot(t, root, args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		out, err := runRoot(t, root, "lint")
		if err != nil {
			t.Fatalf("lint after %v: %v\n%s", args, err, out)
		}
		if out != "" {
			t.Errorf("lint after %v =\n%s\nwant clean", args, out)
		}
	}
}
