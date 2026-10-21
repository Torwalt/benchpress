package board

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Torwalt/benchpress/internal/next"
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

func TestNextTitleEscaping(t *testing.T) {
	cases := []struct {
		title string
		slug  string
		label string
	}{
		{"Fix [thing]", "fix-thing", `Fix \[thing\]`},
		{"A [b](c.md)", "a-b-c-md", `A \[b\](c.md)`},
		{`Back\slash [x]`, "back-slash-x", `Back\\slash \[x\]`},
	}
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			b := loadFixture(t, map[string]string{
				"c.md": topicBody("bug", "maps", "open", "C"),
			}, "# Next\n\n")
			if _, err := b.New(c.title, "bug", "maps", true); err != nil {
				t.Fatalf("New: %v", err)
			}
			body := readFile(t, b.Root, "docs/workbench/"+c.slug+".md")
			if !strings.Contains(body, "\n# "+c.title+"\n") {
				t.Errorf("topic title changed:\n%s", body)
			}
			want := "[" + c.label + "](" + c.slug + ".md)"
			if items := next.Parse([]byte(readFile(t, b.Root, "docs/workbench/NEXT.md"))).Items; len(items) != 1 || items[0].Text != want || items[0].Target != c.slug+".md" {
				t.Fatalf("Next after New = %+v, want text %q target %q", items, want, c.slug+".md")
			}

			var err error
			if b, err = Load(b.Root, b.Config); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if err := b.NextRemove(c.slug); err != nil {
				t.Fatalf("NextRemove: %v", err)
			}
			if items := next.Parse([]byte(readFile(t, b.Root, "docs/workbench/NEXT.md"))).Items; len(items) != 0 {
				t.Errorf("Next after removal = %+v, want empty", items)
			}
			if err := b.NextAdd(c.slug, 0); err != nil {
				t.Fatalf("NextAdd: %v", err)
			}
			if items := next.Parse([]byte(readFile(t, b.Root, "docs/workbench/NEXT.md"))).Items; len(items) != 1 || items[0].Target != c.slug+".md" {
				t.Errorf("Next after NextAdd = %+v, want target %q", items, c.slug+".md")
			}
			if _, err := b.Remove(c.slug); err != nil {
				t.Fatalf("Remove: %v", err)
			}
			if items := next.Parse([]byte(readFile(t, b.Root, "docs/workbench/NEXT.md"))).Items; len(items) != 0 {
				t.Errorf("Next after rm = %+v, want empty", items)
			}
			if _, err := os.Stat(filepath.Join(b.Root, "docs/workbench/c.md")); err != nil {
				t.Errorf("c.md went away: %v", err)
			}
		})
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

func TestWritesPreserveMode(t *testing.T) {
	const mode fs.FileMode = 0o600
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [A](a.md)\n")
	topicPath := filepath.Join(b.Root, "docs", "workbench", "a.md")
	nextPath := b.nextPath()
	for _, name := range []string{topicPath, nextPath} {
		if err := os.Chmod(name, mode); err != nil {
			t.Fatalf("chmod %s: %v", name, err)
		}
	}

	if err := b.SetState("a", "settled", ""); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	if err := b.NextAdd("b", 0); err != nil {
		t.Fatalf("NextAdd: %v", err)
	}
	for _, name := range []string{topicPath, nextPath} {
		info, err := os.Stat(name)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if got := info.Mode().Perm(); got != mode {
			t.Errorf("%s mode = %v, want %v", filepath.Base(name), got, mode)
		}
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
		{"negative position", "b", -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := readFile(t, b.Root, "docs/workbench/NEXT.md")
			items := append([]next.Item(nil), b.Next.Items...)
			err := b.NextAdd(c.slug, c.at)
			if err == nil {
				t.Fatalf("NextAdd(%q, %d) succeeded, want error", c.slug, c.at)
			}
			if c.at != 0 && !errors.Is(err, next.ErrPosition) {
				t.Errorf("NextAdd(%q, %d) error = %v, want ErrPosition", c.slug, c.at, err)
			}
			if got := readFile(t, b.Root, "docs/workbench/NEXT.md"); got != before {
				t.Errorf("NEXT.md changed on failure:\n%s", got)
			}
			if !reflect.DeepEqual(b.Next.Items, items) {
				t.Errorf("Next.Items changed on failure: %+v", b.Next.Items)
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

type capturedFile struct {
	exists bool
	data   string
	mode   fs.FileMode
}

func captureFile(t *testing.T, name string) capturedFile {
	t.Helper()
	info, err := os.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return capturedFile{}
	}
	if err != nil {
		t.Fatalf("stat %s: %v", name, err)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return capturedFile{exists: true, data: string(data), mode: info.Mode().Perm()}
}

func assertFile(t *testing.T, name string, want capturedFile) {
	t.Helper()
	got := captureFile(t, name)
	if got.exists != want.exists {
		t.Fatalf("%s exists = %v, want %v", name, got.exists, want.exists)
	}
	if got.data != want.data {
		t.Errorf("%s bytes = %q, want %q", name, got.data, want.data)
	}
	if got.mode != want.mode {
		t.Errorf("%s mode = %v, want %v", name, got.mode, want.mode)
	}
}

var errNextWrite = errors.New("injected Next write failure")

func TestNewNextWriteFailureRollsBack(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "")
	topicPath := filepath.Join(b.Root, "docs", "workbench", "a-graphics-preset.md")
	nextPath := b.nextPath()

	beforeTopic := captureFile(t, topicPath)
	beforeNext := captureFile(t, nextPath)
	beforeNextMemory := cloneNext(b.Next)

	b.writeNextFile = func(string, []byte, fs.FileMode) error { return errNextWrite }

	if _, err := b.New("A graphics preset", "idea", "economy", true); !errors.Is(err, errNextWrite) {
		t.Fatalf("New error = %v, want %v", err, errNextWrite)
	}
	assertFile(t, topicPath, beforeTopic)
	assertFile(t, nextPath, beforeNext)
	if !reflect.DeepEqual(b.Next, beforeNextMemory) {
		t.Errorf("Next after rollback = %+v, want %+v", b.Next, beforeNextMemory)
	}

	b.writeNextFile = nil
	if _, err := b.New("A graphics preset", "idea", "economy", true); err != nil {
		t.Fatalf("retry New: %v", err)
	}
	if _, err := os.Stat(topicPath); err != nil {
		t.Errorf("retried topic missing: %v", err)
	}
	if got, want := readFile(t, b.Root, "docs/workbench/NEXT.md"), "# Next\n\n1. [A graphics preset](a-graphics-preset.md)\n"; got != want {
		t.Errorf("NEXT.md after retry = %q, want %q", got, want)
	}
}

func TestRemoveNextWriteFailureRollsBack(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
		"b.md": topicBody("bug", "maps", "open", "B"),
	}, "# Next\n\n1. [B](b.md)\n")
	topicPath := filepath.Join(b.Root, "docs", "workbench", "b.md")
	nextPath := b.nextPath()
	if err := os.Chmod(topicPath, 0o600); err != nil {
		t.Fatalf("chmod topic: %v", err)
	}
	if err := os.Chmod(nextPath, 0o640); err != nil {
		t.Fatalf("chmod Next: %v", err)
	}

	beforeTopic := captureFile(t, topicPath)
	beforeNext := captureFile(t, nextPath)
	beforeNextMemory := cloneNext(b.Next)

	b.writeNextFile = func(string, []byte, fs.FileMode) error { return errNextWrite }

	if _, err := b.Remove("b"); !errors.Is(err, errNextWrite) {
		t.Fatalf("Remove error = %v, want %v", err, errNextWrite)
	}
	assertFile(t, topicPath, beforeTopic)
	assertFile(t, nextPath, beforeNext)
	if !reflect.DeepEqual(b.Next, beforeNextMemory) {
		t.Errorf("Next after rollback = %+v, want %+v", b.Next, beforeNextMemory)
	}

	b.writeNextFile = nil
	linked, err := b.Remove("b")
	if err != nil {
		t.Fatalf("retry Remove: %v", err)
	}
	if len(linked) != 0 {
		t.Errorf("linked = %v, want none", linked)
	}
	if _, err := os.Stat(topicPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("topic still exists after retry: %v", err)
	}
	if got, want := readFile(t, b.Root, "docs/workbench/NEXT.md"), "# Next\n\n"; got != want {
		t.Errorf("NEXT.md after retry = %q, want %q", got, want)
	}
}
