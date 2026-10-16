package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Presentation on raised ground](raised-ground.md)\n")
	writeFile(t, root, "work/raised-ground.md",
		"---\nkind: design\narea: maps\nstate: planned\nplan: docs/plans/P.md\n---\n\n# Presentation on raised ground\n")
	writeFile(t, root, "work/drafts-strand.md",
		"---\nkind: design\narea: economy\nstate: open\n---\n\n# Drafts strand, and the player cannot help them\n")
	writeFile(t, root, "work/bad.md", "# No front matter\n")
	writeFile(t, root, "docs/plans/P.md", "plan\n")
	return root
}

// runLS runs the root command with cwd set to root and returns its output.
func runLS(t *testing.T, root string, args ...string) (string, error) {
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
	cmd.SetArgs(append([]string{"ls"}, args...))
	err = cmd.Execute()
	return out.String(), err
}

func TestLS(t *testing.T) {
	root := fixture(t)
	out, err := runLS(t, root)
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	for _, want := range []string{"Next", "raised-ground", "drafts-strand", "docs/plans/P.md (worktree)", "bad"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	positions := []int{
		strings.Index(out, "Next"),
		strings.Index(out, "raised-ground"),
		strings.Index(out, "economy"),
		strings.Index(out, "drafts-strand"),
		strings.Index(out, "bad"),
	}
	for i := 1; i < len(positions); i++ {
		if positions[i-1] > positions[i] {
			t.Errorf("output out of order at %d:\n%s", i, out)
		}
	}
}

func TestLSJSON(t *testing.T) {
	root := fixture(t)
	out, err := runLS(t, root, "--json")
	if err != nil {
		t.Fatalf("ls --json: %v\n%s", err, out)
	}
	var got struct {
		Next   []string `json:"next"`
		Topics []struct {
			Slug      string   `json:"slug"`
			Path      string   `json:"path"`
			Title     string   `json:"title"`
			Kind      string   `json:"kind"`
			Area      string   `json:"area"`
			State     string   `json:"state"`
			Plan      string   `json:"plan"`
			PlanFound []string `json:"plan_found"`
		} `json:"topics"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(got.Next) != 1 || got.Next[0] != "raised-ground" {
		t.Errorf("next = %v, want [raised-ground]", got.Next)
	}
	wantOrder := []string{"raised-ground", "drafts-strand", "bad"}
	if len(got.Topics) != len(wantOrder) {
		t.Fatalf("topics = %+v, want %v", got.Topics, wantOrder)
	}
	for i, want := range wantOrder {
		if got.Topics[i].Slug != want {
			t.Errorf("topics[%d].slug = %q, want %q", i, got.Topics[i].Slug, want)
		}
	}
	planned := got.Topics[0]
	if planned.State != "planned" || planned.Kind != "design" || planned.Area != "maps" {
		t.Errorf("planned topic = %+v", planned)
	}
	if planned.Path != "work/raised-ground.md" {
		t.Errorf("path = %q, want work/raised-ground.md", planned.Path)
	}
	if planned.Plan != "docs/plans/P.md" {
		t.Errorf("plan = %q", planned.Plan)
	}
	if len(planned.PlanFound) == 0 || planned.PlanFound[0] != "worktree" {
		t.Errorf("plan_found = %v, want worktree first", planned.PlanFound)
	}
	if bad := got.Topics[2]; bad.Slug != "bad" || bad.State != "" {
		t.Errorf("broken entry = %+v", bad)
	}
}
