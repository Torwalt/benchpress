package board

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestNew(t *testing.T) {
	b := loadFixture(t, nil, "")
	rel, err := b.New("A graphics preset!", "idea", "economy", false)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rel != "docs/workbench/a-graphics-preset.md" {
		t.Errorf("path = %q", rel)
	}
	want := "---\nkind: idea\narea: economy\nstate: open\n---\n\n# A graphics preset!\n\n**Now:**\n\n**Open:**\n"
	if got := readFile(t, b.Root, rel); got != want {
		t.Errorf("file =\n%q\nwant\n%q", got, want)
	}
}

func TestNewNext(t *testing.T) {
	b := loadFixture(t, nil, "")
	if _, err := b.New("A graphics preset", "idea", "economy", true); err != nil {
		t.Fatalf("New: %v", err)
	}
	want := "# Next\n\n1. [A graphics preset](a-graphics-preset.md)\n"
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != want {
		t.Errorf("NEXT.md =\n%q\nwant\n%q", got, want)
	}
}

func TestNewFailures(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"taken.md": topicBody("bug", "maps", "open", "Taken"),
	}, "# Next\n")
	cases := []struct {
		name              string
		title, kind, area string
	}{
		{"existing file", "Taken", "bug", "maps"},
		{"disallowed kind", "A new one", "nope", "maps"},
		{"disallowed area", "A new one", "bug", "nope"},
		{"no slug", "!!!", "bug", "maps"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := b.New(c.title, c.kind, c.area, false); err == nil {
				t.Fatalf("New(%q, %q, %q) succeeded, want error", c.title, c.kind, c.area)
			}
		})
	}
}

func TestSetState(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "# Next\n")

	if err := b.SetState("a", "planned", "docs/plans/P.md"); err != nil {
		t.Fatalf("SetState planned: %v", err)
	}
	planned := readFile(t, b.Root, "docs/workbench/a.md")
	for _, want := range []string{"state: planned\n", "plan: docs/plans/P.md\n"} {
		if !strings.Contains(planned, want) {
			t.Errorf("planned file missing %q:\n%s", want, planned)
		}
	}

	if err := b.SetState("a.md", "settled", ""); err != nil {
		t.Fatalf("SetState settled: %v", err)
	}
	settled := readFile(t, b.Root, "docs/workbench/a.md")
	if !strings.Contains(settled, "state: settled\n") || strings.Contains(settled, "plan:") {
		t.Errorf("settled file = %q", settled)
	}
	if settled != topicBody("bug", "maps", "settled", "A") {
		t.Errorf("settled bytes changed beyond the front matter:\n%q", settled)
	}
}

func TestSetStateFailures(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "# Next\n")
	cases := []struct {
		name        string
		slug, state string
		plan        string
	}{
		{"unknown slug", "nope", "settled", ""},
		{"planned without plan", "a", "planned", ""},
		{"bad state", "a", "done", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := b.SetState(c.slug, c.state, c.plan); err == nil {
				t.Fatalf("SetState(%q, %q, %q) succeeded, want error", c.slug, c.state, c.plan)
			}
		})
	}
}

func TestNextAdd(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
		"c.md": topicBody("bug", "maps", "open", "C"),
	}, "# Next\n\n1. [A](a.md)\n")

	if err := b.NextAdd("b", 0); err != nil {
		t.Fatalf("NextAdd end: %v", err)
	}
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != "# Next\n\n1. [A](a.md)\n2. [B](b.md)\n" {
		t.Errorf("after append:\n%s", got)
	}
	if err := b.NextAdd("c", 1); err != nil {
		t.Fatalf("NextAdd at: %v", err)
	}
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != "# Next\n\n1. [C](c.md)\n2. [A](a.md)\n3. [B](b.md)\n" {
		t.Errorf("after insert:\n%s", got)
	}
}

func TestNextAddFailures(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [A](a.md)\n")
	cases := []struct {
		name string
		slug string
		at   int
	}{
		{"unknown slug", "nope", 0},
		{"already in Next", "a", 0},
		{"position out of range", "b", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := b.NextAdd(c.slug, c.at); err == nil {
				t.Fatalf("NextAdd(%q, %d) succeeded, want error", c.slug, c.at)
			}
		})
	}
}

func TestNextRemove(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [A](a.md)\n2. [B](b.md)\n")
	if err := b.NextRemove("a"); err != nil {
		t.Fatalf("NextRemove: %v", err)
	}
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != "# Next\n\n1. [B](b.md)\n" {
		t.Errorf("after remove:\n%s", got)
	}
}

func TestNextRemoveFailures(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [A](a.md)\n")
	if err := b.NextRemove("nope"); err == nil {
		t.Error("NextRemove(unknown) succeeded, want error")
	}
	if err := b.NextRemove("b"); err == nil {
		t.Error("NextRemove(not in Next) succeeded, want error")
	}
}

func TestNextMove(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
		"c.md": topicBody("bug", "maps", "open", "C"),
	}, "# Next\n\n1. [A](a.md)\n2. [B](b.md)\n3. [C](c.md)\n")
	if err := b.NextMove("c", 1); err != nil {
		t.Fatalf("NextMove: %v", err)
	}
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != "# Next\n\n1. [C](c.md)\n2. [A](a.md)\n3. [B](b.md)\n" {
		t.Errorf("after move:\n%s", got)
	}
}

func TestNextMoveFailures(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [A](a.md)\n")
	for _, c := range []struct {
		name string
		slug string
		at   int
	}{{"unknown slug", "nope", 1}, {"not in Next", "b", 1}, {"out of range", "a", 3}} {
		t.Run(c.name, func(t *testing.T) {
			if err := b.NextMove(c.slug, c.at); err == nil {
				t.Fatalf("NextMove(%q, %d) succeeded, want error", c.slug, c.at)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\nSee [B](b.md).\n",
		"b.md": topicBody("bug", "maps", "open", "B"),
		"c.md": topicBody("bug", "maps", "open", "C"),
	}, "# Next\n\n1. [B](b.md)\n")
	linked, err := b.Remove("b")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if want := []string{"docs/workbench/a.md"}; !reflect.DeepEqual(linked, want) {
		t.Errorf("linked = %v, want %v", linked, want)
	}
	if _, err := os.Stat(filepath.Join(b.Root, "docs/workbench/b.md")); !os.IsNotExist(err) {
		t.Errorf("b.md still exists (err=%v)", err)
	}
	if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != "# Next\n\n" {
		t.Errorf("after rm:\n%q", got)
	}
	if _, err := b.Remove("nope"); err == nil {
		t.Error("Remove(unknown) succeeded, want error")
	}
}
