package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/config"
	"github.com/Torwalt/benchpress/internal/lint"
)

func writeTopic(t *testing.T, root, name, body string) {
	t.Helper()
	full := filepath.Join(root, "docs", "workbench", name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func topicText(kind, area, state, title string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state +
		"\n---\n\n# " + title + "\n\n**Now:** the build does a thing.\n"
}

// fixtureBoard is five topics across two areas and three states, with one in
// Next.
func fixtureBoard(t *testing.T) *board.Board {
	t.Helper()
	root := t.TempDir()
	writeTopic(t, root, "raised-ground.md", topicText("design", "maps", "planned", "Presentation on raised ground"))
	writeTopic(t, root, "graphics-preset.md", topicText("idea", "economy", "open", "A graphics preset"))
	writeTopic(t, root, "drafts-strand.md", topicText("design", "economy", "open", "Drafts strand, and the player cannot help them"))
	writeTopic(t, root, "taxes.md", topicText("change", "economy", "settled", "Taxes"))
	writeTopic(t, root, "war-table.md", topicText("design", "maps", "settled", "The war table"))
	writeTopic(t, root, "NEXT.md", "# Next\n\n1. [Presentation on raised ground](raised-ground.md)\n")
	cfg := &config.Config{
		Dir:   "docs/workbench",
		Kinds: []string{"bug", "change", "design", "idea"},
		Areas: []string{"economy", "maps"},
	}
	b, err := board.Load(root, cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

// newTestModel builds a sized board model.
func newTestModel(t *testing.T, opts Options) *Model {
	t.Helper()
	if opts.Board == nil {
		opts.Board = fixtureBoard(t)
	}
	m := New(opts)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func pressRune(m *Model, r rune) { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}) }

func pressType(m *Model, r tea.KeyType) { m.Update(tea.KeyMsg{Type: r}) }

func typeText(m *Model, s string) {
	for _, r := range s {
		pressRune(m, r)
	}
}

func wantSlug(t *testing.T, m *Model, want string) {
	t.Helper()
	e, ok := m.selectedEntry()
	if !ok {
		t.Fatalf("no topic selected, want %q", want)
	}
	if e.Slug != want {
		t.Errorf("selected = %q, want %q", e.Slug, want)
	}
}

func groupLabels(m *Model) []string {
	var out []string
	for _, r := range m.rows {
		if r.kind == rowGroup {
			out = append(out, r.label)
		}
	}
	return out
}

func TestSelectionStartsOnFirstTopic(t *testing.T) {
	m := newTestModel(t, Options{})
	wantSlug(t, m, "raised-ground")
}

func TestSelectionMoves(t *testing.T) {
	m := newTestModel(t, Options{})
	pressRune(m, 'j')
	wantSlug(t, m, "graphics-preset")
	pressRune(m, 'k')
	wantSlug(t, m, "raised-ground")
	pressType(m, tea.KeyUp)
	wantSlug(t, m, "raised-ground")
	pressType(m, tea.KeyDown)
	wantSlug(t, m, "graphics-preset")
	pressRune(m, 'G')
	wantSlug(t, m, "war-table")
	pressRune(m, 'g')
	wantSlug(t, m, "raised-ground")
}

func TestFilterNarrowsAndClears(t *testing.T) {
	m := newTestModel(t, Options{})
	pressRune(m, '/')
	typeText(m, "graphics")
	wantSlug(t, m, "graphics-preset")
	if strings.Contains(m.View(), "The war table") {
		t.Errorf("filtered view still shows an unmatched topic:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "A graphics preset") {
		t.Errorf("filtered view missing the match:\n%s", m.View())
	}
	pressType(m, tea.KeyEsc)
	if m.filter != "" {
		t.Errorf("filter = %q, want empty", m.filter)
	}
	if !strings.Contains(m.View(), "The war table") {
		t.Errorf("cleared view missing war table:\n%s", m.View())
	}
}

func TestFilterMatchesSlugKindAndArea(t *testing.T) {
	for _, q := range []string{"drafts", "change", "economy"} {
		m := newTestModel(t, Options{})
		pressRune(m, '/')
		typeText(m, q)
		if _, ok := m.selectedEntry(); !ok {
			t.Errorf("filter %q selected nothing", q)
		}
	}
}

func TestTabCyclesGrouping(t *testing.T) {
	m := newTestModel(t, Options{})
	if m.grouping != "area" {
		t.Fatalf("grouping = %q, want area", m.grouping)
	}
	if got := groupLabels(m); strings.Join(got, ",") != "economy,maps" {
		t.Errorf("area groups = %v, want [economy maps]", got)
	}
	pressType(m, tea.KeyTab)
	if m.grouping != "state" {
		t.Fatalf("grouping = %q, want state", m.grouping)
	}
	if got := groupLabels(m); strings.Join(got, ",") != "open,settled" {
		t.Errorf("state groups = %v, want [open settled]", got)
	}
	pressType(m, tea.KeyTab)
	if m.grouping != "kind" {
		t.Fatalf("grouping = %q, want kind", m.grouping)
	}
	if got := groupLabels(m); strings.Join(got, ",") != "change,design,idea" {
		t.Errorf("kind groups = %v, want [change design idea]", got)
	}
	pressType(m, tea.KeyTab)
	if m.grouping != "area" {
		t.Errorf("grouping = %q, want area", m.grouping)
	}
}

func TestNarrowHidesPreviewAndEnterToggles(t *testing.T) {
	m := newTestModel(t, Options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	if !m.narrow {
		t.Fatalf("width 80 should be narrow")
	}
	if m.narrowPreview {
		t.Fatalf("preview should start hidden when narrow")
	}
	pressType(m, tea.KeyEnter)
	if !m.narrowPreview {
		t.Errorf("enter should show the preview when narrow")
	}
	pressType(m, tea.KeyEnter)
	if m.narrowPreview {
		t.Errorf("enter should hide the preview again")
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if m.narrow {
		t.Errorf("width 120 should not be narrow")
	}
}

func TestViewNamesSelectedTitle(t *testing.T) {
	m := newTestModel(t, Options{})
	if !strings.Contains(m.View(), "Presentation on raised ground") {
		t.Errorf("view missing first title:\n%s", m.View())
	}
	pressRune(m, 'j')
	if !strings.Contains(m.View(), "A graphics preset") {
		t.Errorf("view missing second title:\n%s", m.View())
	}
}

func TestLintSummary(t *testing.T) {
	problems := []lint.Problem{
		{Severity: lint.Error},
		{Severity: lint.Error},
		{Severity: lint.Warning},
	}
	m := newTestModel(t, Options{Problems: problems})
	if !strings.Contains(m.View(), "2 errors, 1 warning") {
		t.Errorf("view missing summary:\n%s", m.View())
	}
	clean := newTestModel(t, Options{})
	if !strings.Contains(clean.View(), "lint clean") {
		t.Errorf("clean view missing 'lint clean':\n%s", clean.View())
	}
}

func TestViewMarksLintErrorRows(t *testing.T) {
	b := fixtureBoard(t)
	problems := []lint.Problem{{Severity: lint.Error, Path: "docs/workbench/taxes.md"}}
	m := newTestModel(t, Options{Board: b, Problems: problems})
	for _, r := range m.rows {
		if r.kind != rowTopic {
			continue
		}
		marked := strings.HasSuffix(strings.TrimSpace(m.topicText(r.entry, "")), "!")
		if r.entry.Slug == "taxes" && !marked {
			t.Errorf("taxes row not marked: %q", m.topicText(r.entry, ""))
		}
		if r.entry.Slug != "taxes" && marked {
			t.Errorf("%s row wrongly marked: %q", r.entry.Slug, m.topicText(r.entry, ""))
		}
	}
}
