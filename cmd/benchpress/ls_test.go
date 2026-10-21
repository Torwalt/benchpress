package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

func filterFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\", \"army\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Open bug](open-bug.md)\n2. [Settled idea](settled-idea.md)\n3. [Open change](open-change.md)\n")
	writeFile(t, root, "work/open-bug.md",
		"---\nkind: bug\narea: economy\nstate: open\n---\n\n# Open bug\n")
	writeFile(t, root, "work/settled-idea.md",
		"---\nkind: idea\narea: maps\nstate: settled\n---\n\n# Settled idea\n")
	writeFile(t, root, "work/open-change.md",
		"---\nkind: change\narea: maps\nstate: open\n---\n\n# Open change\n")
	writeFile(t, root, "work/planned-design.md",
		"---\nkind: design\narea: economy\nstate: planned\nplan: docs/plans/P.md\n---\n\n# Planned design\n")
	writeFile(t, root, "work/open-army.md",
		"---\nkind: bug\narea: army\nstate: open\n---\n\n# Open army\n")
	writeFile(t, root, "docs/plans/P.md", "plan\n")
	return root
}

// textListing returns the slugs under Next and the slugs in the grouped rows.
func textListing(out string) (next, topics []string) {
	next, topics = []string{}, []string{}
	inNext := false
	for _, line := range strings.Split(out, "\n") {
		switch {
		case line == "Next":
			inNext = true
		case line == "":
			inNext = false
		case strings.HasPrefix(line, "  "):
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			if inNext {
				next = append(next, fields[0])
			} else {
				topics = append(topics, fields[0])
			}
		}
	}
	return next, topics
}

func TestLSFilters(t *testing.T) {
	root := filterFixture(t)
	cases := []struct {
		name    string
		args    []string
		next    []string
		entries []string
	}{
		{
			name:    "state",
			args:    []string{"--state", "open"},
			next:    []string{"open-bug", "open-change"},
			entries: []string{"open-bug", "open-change", "open-army"},
		},
		{
			name:    "repeated states",
			args:    []string{"--state", "settled", "--state", "planned"},
			next:    []string{"settled-idea"},
			entries: []string{"settled-idea", "planned-design"},
		},
		{
			name:    "kind",
			args:    []string{"--kind", "bug"},
			next:    []string{"open-bug"},
			entries: []string{"open-bug", "open-army"},
		},
		{
			name:    "repeated kinds",
			args:    []string{"--kind", "bug", "--kind", "idea"},
			next:    []string{"open-bug", "settled-idea"},
			entries: []string{"open-bug", "settled-idea", "open-army"},
		},
		{
			name:    "area",
			args:    []string{"--area", "maps"},
			next:    []string{"settled-idea", "open-change"},
			entries: []string{"settled-idea", "open-change"},
		},
		{
			name:    "repeated areas",
			args:    []string{"--area", "economy", "--area", "army"},
			next:    []string{"open-bug"},
			entries: []string{"open-bug", "planned-design", "open-army"},
		},
		{
			name:    "different flags intersect",
			args:    []string{"--state", "open", "--area", "maps"},
			next:    []string{"open-change"},
			entries: []string{"open-change"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := runLS(t, root, c.args...)
			if err != nil {
				t.Fatalf("ls: %v\n%s", err, out)
			}
			next, topics := textListing(out)
			if !reflect.DeepEqual(next, c.next) {
				t.Errorf("text next = %v, want %v:\n%s", next, c.next, out)
			}
			if want := c.entries[len(c.next):]; !reflect.DeepEqual(topics, want) {
				t.Errorf("text topics = %v, want %v:\n%s", topics, want, out)
			}

			jsonOut, err := runLS(t, root, append(c.args, "--json")...)
			if err != nil {
				t.Fatalf("ls --json: %v\n%s", err, jsonOut)
			}
			var got struct {
				Next   []string `json:"next"`
				Topics []struct {
					Slug string `json:"slug"`
				} `json:"topics"`
			}
			if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
				t.Fatalf("unmarshal: %v\n%s", err, jsonOut)
			}
			if !reflect.DeepEqual(got.Next, c.next) {
				t.Errorf("json next = %v, want %v", got.Next, c.next)
			}
			slugs := make([]string, len(got.Topics))
			for i, topic := range got.Topics {
				slugs[i] = topic.Slug
			}
			if !reflect.DeepEqual(slugs, c.entries) {
				t.Errorf("json topics = %v, want %v", slugs, c.entries)
			}
		})
	}
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

func TestLSNextOrder(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [Beta second](second.md)\n2. [Alpha first](first.md)\n")
	writeFile(t, root, "work/first.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Alpha first\n")
	writeFile(t, root, "work/second.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Beta second\n")

	out, err := runLS(t, root)
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	first := strings.Index(out, "  first  ")
	second := strings.Index(out, "  second  ")
	if first < 0 || second < 0 {
		t.Fatalf("entries missing from output:\n%s", out)
	}
	if second > first {
		t.Errorf("Next entries out of authored order:\n%s", out)
	}

	jsonOut, err := runLS(t, root, "--json")
	if err != nil {
		t.Fatalf("ls --json: %v\n%s", err, jsonOut)
	}
	var got struct {
		Next   []string `json:"next"`
		Topics []struct {
			Slug string `json:"slug"`
		} `json:"topics"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, jsonOut)
	}
	want := []string{"second", "first"}
	if !reflect.DeepEqual(got.Next, want) {
		t.Errorf("next = %v, want %v", got.Next, want)
	}
	topicSlugs := make([]string, len(got.Topics))
	for i, topic := range got.Topics {
		topicSlugs[i] = topic.Slug
	}
	if !reflect.DeepEqual(topicSlugs, want) {
		t.Errorf("topics = %v, want %v", topicSlugs, want)
	}
}

func TestLSInvalidNextTarget(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	writeFile(t, root, ".benchpress.toml", "dir = \"work\"\nareas = [\"economy\", \"maps\"]\n")
	writeFile(t, root, "work/NEXT.md", "# Next\n\n1. [A](sub/a.md)\n")
	writeFile(t, root, "work/a.md", "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n")

	out, err := runLS(t, root)
	if err != nil {
		t.Fatalf("ls: %v\n%s", err, out)
	}
	if strings.Contains(out, "Next") {
		t.Errorf("invalid Next target listed under Next:\n%s", out)
	}
	row := "  a  open  bug  A"
	if !strings.Contains(out, "maps") || !strings.Contains(out, row) {
		t.Fatalf("a not in its area group:\n%s", out)
	}
	if strings.Index(out, "maps") > strings.Index(out, row) {
		t.Errorf("a listed before its area group:\n%s", out)
	}

	jsonOut, err := runLS(t, root, "--json")
	if err != nil {
		t.Fatalf("ls --json: %v\n%s", err, jsonOut)
	}
	var got struct {
		Next   []string `json:"next"`
		Topics []struct {
			Slug string `json:"slug"`
			Area string `json:"area"`
		} `json:"topics"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, jsonOut)
	}
	if len(got.Next) != 0 {
		t.Errorf("next = %v, want empty", got.Next)
	}
	if len(got.Topics) != 1 || got.Topics[0].Slug != "a" || got.Topics[0].Area != "maps" {
		t.Errorf("topics = %+v, want a in maps", got.Topics)
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
