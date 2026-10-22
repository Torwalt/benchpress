package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/config"
	"github.com/Torwalt/benchpress/internal/lint"
)

func writeTopic(t *testing.T, root, name, body string) {
	t.Helper()
	writeTopicIn(t, root, "docs/workbench", name, body)
}

// writeTopicIn writes a topic file directly inside a workbench directory.
func writeTopicIn(t *testing.T, root, dir, name, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(dir), name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// writeConfig writes a .benchpress.toml so a reload can read it back.
func writeConfig(t *testing.T, root, dir string, kinds, areas []string) {
	t.Helper()
	quote := func(values []string) string {
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		return strings.Join(quoted, ", ")
	}
	body := fmt.Sprintf("dir = %q\nkinds = [%s]\nareas = [%s]\n", dir, quote(kinds), quote(areas))
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// loadConfig loads a fixture's configuration from disk.
func loadConfig(t *testing.T, root string) *config.Config {
	t.Helper()
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func topicText(kind, area, state, title string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state +
		"\n---\n\n# " + title + "\n\n**Now:** the build does a thing.\n"
}

// topicWithBody is topicText with a caller-chosen body.
func topicWithBody(kind, area, state, title, body string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state +
		"\n---\n\n# " + title + "\n\n" + body
}

// previewBody puts phrase in a body with filler paragraphs long enough to
// scroll the preview.
func previewBody(phrase string, paragraphs int) string {
	var b strings.Builder
	b.WriteString("**Now:** " + phrase + ".\n")
	for i := 0; i < paragraphs; i++ {
		fmt.Fprintf(&b, "\nFiller paragraph %d, apart from the marker phrase.\n", i)
	}
	return b.String()
}

// ansiEscape matches the terminal styling glamour puts around words.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plainView drops terminal styling so a test can look for rendered text.
func plainView(s string) string { return ansiEscape.ReplaceAllString(s, "") }

// previewBoard is two topics whose bodies carry distinct phrases absent from
// their titles, the first long enough to scroll.
func previewBoard(t *testing.T) *board.Board {
	t.Helper()
	root := t.TempDir()
	writeTopic(t, root, "alpha.md", topicWithBody("design", "economy", "open", "Alpha topic",
		previewBody("the midnight quokka phrase", 40)))
	writeTopic(t, root, "beta.md", topicWithBody("design", "economy", "open", "Beta topic",
		previewBody("the dawn lorikeet phrase", 1)))
	writeTopic(t, root, "NEXT.md", "# Next\n\n1. [Alpha topic](alpha.md)\n2. [Beta topic](beta.md)\n")
	writeConfig(t, root, "docs/workbench", []string{"bug", "change", "design", "idea"}, []string{"economy", "maps"})
	b, err := board.Load(root, loadConfig(t, root))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
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
	writeConfig(t, root, "docs/workbench", []string{"bug", "change", "design", "idea"}, []string{"economy", "maps"})
	b, err := board.Load(root, loadConfig(t, root))
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
		if r.kind == rowGroup && r.label != nextGroupLabel {
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

func TestViewNamesNextSection(t *testing.T) {
	m := newTestModel(t, Options{})
	writeTopic(t, m.board.Root, "NEXT.md",
		"# Next\n\n1. [Presentation on raised ground](raised-ground.md)\n2. [A graphics preset](graphics-preset.md)\n")
	b, err := board.Load(m.board.Root, m.board.Config)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m = newTestModel(t, Options{Board: b})
	wantSlug(t, m, "raised-ground")

	heading, first := -1, -1
	for i, r := range m.rows {
		if r.kind == rowGroup && r.label == nextGroupLabel {
			heading = i
		}
		if r.kind == rowNext && first < 0 {
			first = i
		}
	}
	if heading < 0 || first < 0 {
		t.Fatalf("rows = %+v, want a Next heading and a Next item", m.rows)
	}
	if heading >= first {
		t.Errorf("Next heading at row %d, first item at row %d; want the heading first", heading, first)
	}
	if m.rows[first].number != 1 || m.rows[first+1].number != 2 {
		t.Errorf("Next numbers = %d,%d; want 1,2", m.rows[first].number, m.rows[first+1].number)
	}
	view := m.View()
	if i, j := strings.Index(view, nextGroupLabel), strings.Index(view, "Presentation on raised ground"); i < 0 || j < 0 || i > j {
		t.Errorf("view does not put the Next heading before its first item:\n%s", view)
	}

	pressRune(m, 'k')
	wantSlug(t, m, "raised-ground")
	if m.rows[m.cursor].kind != rowNext {
		t.Errorf("selection rests on %v; want a Next item", m.rows[m.cursor].kind)
	}
	pressRune(m, 'g')
	if m.rows[m.cursor].kind != rowNext {
		t.Errorf("g selects %v; want a Next item", m.rows[m.cursor].kind)
	}

	pressRune(m, '/')
	typeText(m, "war")
	for _, r := range m.rows {
		if r.kind == rowGroup && r.label == nextGroupLabel {
			t.Errorf("filter with no Next match kept the Next heading: %+v", m.rows)
		}
	}
	if strings.Contains(m.View(), nextGroupLabel) {
		t.Errorf("filtered view still shows the Next heading:\n%s", m.View())
	}
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

// visibleSlugs is every selectable row's slug, sorted.
func visibleSlugs(m *Model) []string {
	var out []string
	for _, r := range m.rows {
		if selectable(r.kind) {
			out = append(out, r.entry.Slug)
		}
	}
	sort.Strings(out)
	return out
}

func TestFilterMatchesSlugKindAndArea(t *testing.T) {
	cases := []struct {
		query string
		want  []string
	}{
		{"raised-ground", []string{"raised-ground"}},
		{"change", []string{"taxes"}},
		{"economy", []string{"drafts-strand", "graphics-preset", "taxes"}},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			m := newTestModel(t, Options{})
			pressRune(m, '/')
			typeText(m, tc.query)
			if got := visibleSlugs(m); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filter %q shows %v, want %v", tc.query, got, tc.want)
			}
			e, ok := m.selectedEntry()
			if !ok {
				t.Fatalf("filter %q selected nothing, want one of %v", tc.query, tc.want)
			}
			if !contains(tc.want, e.Slug) {
				t.Errorf("filter %q selected %q, want one of %v", tc.query, e.Slug, tc.want)
			}
		})
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

func TestHelpExplainsNarrowEnter(t *testing.T) {
	m := newTestModel(t, Options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	pressRune(m, '?')
	help := plainView(m.View())
	if !strings.Contains(help, "e enter") || !strings.Contains(help, "open the topic in the editor") {
		t.Errorf("help does not explain the editor binding:\n%s", help)
	}
	if !strings.Contains(help, "enter toggles the preview below 100 columns") {
		t.Errorf("help does not explain Enter's narrow-layout exception:\n%s", help)
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

func TestPreviewShowsSelectedBody(t *testing.T) {
	m := newTestModel(t, Options{Board: previewBoard(t)})
	wantSlug(t, m, "alpha")
	const alphaPhrase = "the midnight quokka phrase"
	const betaPhrase = "the dawn lorikeet phrase"

	view := plainView(m.View())
	if !strings.Contains(view, alphaPhrase) {
		t.Fatalf("wide view missing the selected body phrase %q:\n%s", alphaPhrase, view)
	}
	if strings.Contains(view, betaPhrase) {
		t.Fatalf("wide view shows the unselected body phrase %q:\n%s", betaPhrase, view)
	}

	pressRune(m, 'j')
	wantSlug(t, m, "beta")
	view = plainView(m.View())
	if !strings.Contains(view, betaPhrase) {
		t.Errorf("view after selecting the second topic missing %q:\n%s", betaPhrase, view)
	}
	if strings.Contains(view, alphaPhrase) {
		t.Errorf("view after selecting the second topic still shows %q:\n%s", alphaPhrase, view)
	}

	pressRune(m, 'k')
	wantSlug(t, m, "alpha")
	if view := plainView(m.View()); !strings.Contains(view, alphaPhrase) {
		t.Errorf("view after returning to the first topic missing %q:\n%s", alphaPhrase, view)
	}
}

func TestPreviewScrollKeys(t *testing.T) {
	m := newTestModel(t, Options{Board: previewBoard(t)})
	wantSlug(t, m, "alpha")
	if m.preview.YOffset != 0 {
		t.Fatalf("initial YOffset = %d, want 0", m.preview.YOffset)
	}

	pressType(m, tea.KeyCtrlD)
	if m.preview.YOffset <= 0 {
		t.Fatalf("ctrl+d left YOffset = %d, want it to increase", m.preview.YOffset)
	}
	first := m.preview.YOffset

	pressType(m, tea.KeyCtrlD)
	if m.preview.YOffset <= first {
		t.Fatalf("second ctrl+d left YOffset = %d, want more than %d", m.preview.YOffset, first)
	}
	down := m.preview.YOffset

	pressType(m, tea.KeyCtrlU)
	if m.preview.YOffset >= down {
		t.Fatalf("ctrl+u left YOffset = %d, want less than %d", m.preview.YOffset, down)
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

// workbenchFiles reads the fixture workbench so a test can prove a key wrote
// nothing.
func workbenchFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	dir := filepath.Join(root, "docs", "workbench")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read workbench: %v", err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		out[e.Name()] = string(data)
	}
	return out
}

// isQuitCmd reports whether executing cmd yields tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// modeCase enters one of the board's modes.
type modeCase struct {
	name  string
	kind  promptKind
	setup func(*Model)
}

// promptCases enters every prompt with its initial input.
func promptCases() []modeCase {
	prompts := []struct {
		name  string
		kind  promptKind
		label string
		value string
	}{
		{"plan prompt", promptPlan, "plan: ", "docs/plans/"},
		{"title prompt", promptNewTitle, "title: ", ""},
		{"kind prompt", promptNewKind, "kind: ", ""},
		{"area prompt", promptNewArea, "area: ", ""},
	}
	cases := make([]modeCase, 0, len(prompts))
	for _, p := range prompts {
		cases = append(cases, modeCase{p.name, p.kind, func(m *Model) {
			m.prompt = p.kind
			m.setInput(p.label, p.value)
			m.input.Focus()
		}})
	}
	return cases
}

func TestQuitFromEveryMode(t *testing.T) {
	cases := []modeCase{
		{"normal", promptNone, func(*Model) {}},
		{"help", promptNone, func(m *Model) { pressRune(m, '?') }},
		{"filter", promptNone, func(m *Model) { pressRune(m, '/') }},
		{"delete confirmation", promptNone, func(m *Model) { pressRune(m, 'd') }},
	}
	cases = append(cases, promptCases()...)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, Options{})
			before := workbenchFiles(t, m.board.Root)
			tc.setup(m)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			if !isQuitCmd(cmd) {
				t.Fatalf("ctrl+c in %s did not return tea.QuitMsg", tc.name)
			}
			if after := workbenchFiles(t, m.board.Root); !reflect.DeepEqual(before, after) {
				t.Errorf("ctrl+c in %s wrote to the workbench", tc.name)
			}
		})
	}
}

func TestQQuitsFromHelpAndConfirmation(t *testing.T) {
	cases := []modeCase{
		{"help", promptNone, func(m *Model) { pressRune(m, '?') }},
		{"delete confirmation", promptNone, func(m *Model) { pressRune(m, 'd') }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, Options{})
			before := workbenchFiles(t, m.board.Root)
			tc.setup(m)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
			if !isQuitCmd(cmd) {
				t.Fatalf("q in %s did not return tea.QuitMsg", tc.name)
			}
			if after := workbenchFiles(t, m.board.Root); !reflect.DeepEqual(before, after) {
				t.Errorf("q in %s wrote to the workbench", tc.name)
			}
		})
	}
}

func TestQIsTextInFilterAndPrompts(t *testing.T) {
	t.Run("filter", func(t *testing.T) {
		m := newTestModel(t, Options{})
		pressRune(m, '/')
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		if isQuitCmd(cmd) {
			t.Fatalf("q quit from the filter instead of being text")
		}
		if m.filter != "q" || !m.filtering {
			t.Errorf("filter = %q, filtering = %v; want %q and true", m.filter, m.filtering, "q")
		}
	})
	for _, tc := range promptCases() {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(t, Options{})
			tc.setup(m)
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
			if isQuitCmd(cmd) {
				t.Fatalf("q quit from the %s instead of being text", tc.name)
			}
			if !strings.Contains(m.input.Value(), "q") {
				t.Errorf("%s input = %q, want it to contain q", tc.name, m.input.Value())
			}
			if m.prompt != tc.kind {
				t.Errorf("%s prompt = %v, want %v", tc.name, m.prompt, tc.kind)
			}
		})
	}
}

func TestFilterBackspaceRune(t *testing.T) {
	m := newTestModel(t, Options{})
	pressRune(m, '/')
	pressRune(m, 'é')
	if m.filter != "é" {
		t.Fatalf("filter = %q, want é", m.filter)
	}
	pressType(m, tea.KeyBackspace)
	if m.filter != "" {
		t.Errorf("filter = %q, want empty after deleting é", m.filter)
	}
	if !utf8.ValidString(m.filter) {
		t.Errorf("filter is not valid UTF-8: %q", m.filter)
	}

	typeText(m, "café")
	pressType(m, tea.KeyBackspace)
	if m.filter != "caf" {
		t.Errorf("filter = %q, want caf after deleting the final rune", m.filter)
	}
	if !utf8.ValidString(m.filter) {
		t.Errorf("filter is not valid UTF-8: %q", m.filter)
	}
}

func TestSubTopicsFollowTheirEpic(t *testing.T) {
	root := t.TempDir()
	writeTopic(t, root, "summoner.md", topicText("design", "maps", "open", "The Summoner"))
	writeTopic(t, root, "juice.md", "---\nkind: change\narea: economy\nstate: settled\nepic: summoner\n---\n\n# Hits have no weight\n")
	writeTopic(t, root, "roads.md", topicText("design", "economy", "open", "Roads"))
	writeTopic(t, root, "NEXT.md", "# Next\n")
	writeConfig(t, root, "docs/workbench", []string{"bug", "change", "design", "idea"}, []string{"economy", "maps"})
	b, err := board.Load(root, loadConfig(t, root))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := newTestModel(t, Options{Board: b})

	rowSlugs := func() []string {
		var out []string
		for _, r := range m.rows {
			if r.kind == rowGroup {
				out = append(out, "["+r.label+"]")
			} else {
				out = append(out, r.entry.Slug)
			}
		}
		return out
	}
	if got, want := rowSlugs(), []string{"[economy]", "roads", "[maps]", "summoner", "juice"}; !reflect.DeepEqual(got, want) {
		t.Errorf("area rows = %v, want %v", got, want)
	}
	if !strings.Contains(plainView(m.View()), "    ◆ change  Hits have no weight") {
		t.Errorf("sub-topic is not indented:\n%s", plainView(m.View()))
	}
	pressType(m, tea.KeyTab)
	if got, want := rowSlugs(), []string{"[open]", "roads", "summoner", "juice"}; !reflect.DeepEqual(got, want) {
		t.Errorf("state rows = %v, want %v", got, want)
	}

	pressRune(m, '/')
	typeText(m, "hits")
	if got, want := rowSlugs(), []string{"[settled]", "juice"}; !reflect.DeepEqual(got, want) {
		t.Errorf("filtered rows = %v, want %v", got, want)
	}
}
