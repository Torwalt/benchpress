package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/config"
)

// worktreeLocator reports every plan path as found, so a reload stays clean.
type worktreeLocator struct{}

func (worktreeLocator) Find(_ context.Context, paths ...string) (map[string][]string, error) {
	found := make(map[string][]string, len(paths))
	for _, p := range paths {
		found[p] = []string{"worktree"}
	}
	return found, nil
}

// loadActionBoard writes topics and NEXT.md under a fresh directory and loads
// them with the usual configuration.
func loadActionBoard(t *testing.T, topics map[string]string, next string) *board.Board {
	t.Helper()
	root := t.TempDir()
	for name, body := range topics {
		writeTopic(t, root, name, body)
	}
	if next != "" {
		writeTopic(t, root, "NEXT.md", next)
	}
	writeConfig(t, root, "docs/workbench", []string{"bug", "change", "design", "idea"}, []string{"economy", "maps"})
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	b, err := board.Load(root, cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func newActionModel(t *testing.T, b *board.Board) *Model {
	t.Helper()
	m := New(Options{Board: b, Locator: worktreeLocator{}})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

func readAt(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestStateKeyCycles(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)
	wantSlug(t, m, "alpha")

	pressRune(m, 's')
	if got := readAt(t, b.Root, "docs/workbench/alpha.md"); !strings.Contains(got, "state: settled\n") {
		t.Fatalf("after s, file =\n%s", got)
	}

	pressRune(m, 's')
	if m.prompt != promptPlan {
		t.Fatalf("prompt = %v, want plan", m.prompt)
	}
	if got := m.input.Value(); got != "docs/plans/" {
		t.Fatalf("plan input = %q, want docs/plans/", got)
	}
	typeText(m, "P.md")
	pressType(m, tea.KeyEnter)
	got := readAt(t, b.Root, "docs/workbench/alpha.md")
	for _, want := range []string{"state: planned\n", "plan: docs/plans/P.md\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("after plan, file missing %q:\n%s", want, got)
		}
	}

	pressRune(m, 's')
	got = readAt(t, b.Root, "docs/workbench/alpha.md")
	if !strings.Contains(got, "state: open\n") || strings.Contains(got, "plan:") {
		t.Errorf("after cycle, file =\n%s", got)
	}
}

func TestPlanPromptCancel(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "settled", "Alpha"),
	}, "")
	m := newActionModel(t, b)
	pressRune(m, 's')
	if m.prompt != promptPlan {
		t.Fatalf("prompt = %v, want plan", m.prompt)
	}
	pressType(m, tea.KeyEsc)
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v, want none after esc", m.prompt)
	}
	if got := readAt(t, b.Root, "docs/workbench/alpha.md"); !strings.Contains(got, "state: settled\n") {
		t.Errorf("cancel changed the file:\n%s", got)
	}
}

func TestNextToggleKey(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)

	pressRune(m, 'n')
	if next := readAt(t, b.Root, "docs/workbench/NEXT.md"); !strings.Contains(next, "[Alpha](alpha.md)") {
		t.Fatalf("after n, NEXT.md =\n%s", next)
	}
	pressRune(m, 'n')
	if next := readAt(t, b.Root, "docs/workbench/NEXT.md"); strings.Contains(next, "alpha.md") {
		t.Errorf("after second n, NEXT.md =\n%s", next)
	}
}

func TestNextMoveKeys(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
		"beta.md":  topicText("design", "maps", "open", "Beta"),
	}, "# Next\n\n1. [Alpha](alpha.md)\n2. [Beta](beta.md)\n")
	m := newActionModel(t, b)
	wantSlug(t, m, "alpha")

	pressRune(m, 'j')
	wantSlug(t, m, "beta")
	pressRune(m, 'K')
	if next := readAt(t, b.Root, "docs/workbench/NEXT.md"); !strings.HasPrefix(next, "# Next\n\n1. [Beta](beta.md)\n2. [Alpha](alpha.md)\n") {
		t.Fatalf("after K, NEXT.md =\n%s", next)
	}

	pressRune(m, 'J')
	if next := readAt(t, b.Root, "docs/workbench/NEXT.md"); !strings.HasPrefix(next, "# Next\n\n1. [Alpha](alpha.md)\n2. [Beta](beta.md)\n") {
		t.Errorf("after J, NEXT.md =\n%s", next)
	}
}

func TestAddKeyPromptsAndWrites(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)

	pressRune(m, 'a')
	if m.prompt != promptNewTitle {
		t.Fatalf("prompt = %v, want title", m.prompt)
	}
	typeText(m, "A graphics preset")
	pressType(m, tea.KeyEnter)
	if m.prompt != promptNewKind {
		t.Fatalf("prompt = %v, want kind", m.prompt)
	}
	typeText(m, "idea")
	pressType(m, tea.KeyEnter)
	if m.prompt != promptNewArea {
		t.Fatalf("prompt = %v, want area", m.prompt)
	}
	typeText(m, "economy")
	pressType(m, tea.KeyEnter)
	if m.prompt != promptNone {
		t.Fatalf("prompt = %v, want none after add", m.prompt)
	}

	body := readAt(t, b.Root, "docs/workbench/a-graphics-preset.md")
	for _, want := range []string{"# A graphics preset\n", "kind: idea\n", "area: economy\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("new topic missing %q:\n%s", want, body)
		}
	}
}

func TestDeleteKeyConfirmsAndRefuses(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
		"beta.md":  topicText("design", "maps", "open", "Beta"),
	}, "")
	m := newActionModel(t, b)

	pressRune(m, 'd')
	if !m.confirming {
		t.Fatalf("d did not ask for confirmation")
	}
	pressRune(m, 'n')
	if m.confirming {
		t.Fatalf("refusal left the confirmation open")
	}
	if _, err := os.Stat(filepath.Join(b.Root, "docs/workbench/alpha.md")); err != nil {
		t.Fatalf("refusal deleted the topic: %v", err)
	}

	pressRune(m, 'd')
	pressRune(m, 'y')
	if _, err := os.Stat(filepath.Join(b.Root, "docs/workbench/alpha.md")); !os.IsNotExist(err) {
		t.Errorf("confirmation did not delete: err=%v", err)
	}
}

func TestOperationFailureShowsErrorAndChangesNothing(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "settled", "Alpha"),
	}, "")
	m := newActionModel(t, b)

	pressRune(m, 's')
	if m.prompt != promptPlan {
		t.Fatalf("prompt = %v, want plan", m.prompt)
	}
	for range "docs/plans/" {
		pressType(m, tea.KeyBackspace)
	}
	pressType(m, tea.KeyEnter)

	if m.status == "" {
		t.Errorf("failure left no error in the status line")
	} else if !strings.Contains(m.View(), m.status) {
		t.Errorf("view does not show the error %q:\n%s", m.status, m.View())
	}
	if got := readAt(t, b.Root, "docs/workbench/alpha.md"); !strings.Contains(got, "state: settled\n") {
		t.Errorf("failed operation changed the file:\n%s", got)
	}
}

func TestReloadKeyPicksUpDisk(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)

	writeTopic(t, b.Root, "gamma.md", topicText("design", "maps", "open", "Gamma"))
	pressRune(m, 'r')
	if !strings.Contains(m.View(), "Gamma") {
		t.Errorf("reload did not pick up a topic added on disk:\n%s", m.View())
	}
}

func TestReloadReadsConfiguration(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)
	wantSlug(t, m, "alpha")

	writeConfig(t, b.Root, "docs/workbench",
		[]string{"bug", "change", "design", "idea", "art"},
		[]string{"economy", "maps", "space"})
	pressRune(m, 'r')
	if !contains(m.board.Config.Kinds, "art") || !contains(m.board.Config.Areas, "space") {
		t.Fatalf("configuration after r = %+v, want the new kind and area", m.board.Config)
	}

	pressRune(m, 'a')
	typeText(m, "An art piece")
	pressType(m, tea.KeyEnter)
	typeText(m, "art")
	pressType(m, tea.KeyEnter)
	typeText(m, "space")
	pressType(m, tea.KeyEnter)

	body := readAt(t, b.Root, "docs/workbench/an-art-piece.md")
	for _, want := range []string{"kind: art\n", "area: space\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("new topic missing %q:\n%s", want, body)
		}
	}
}

func TestReloadSwitchesConfiguredDirectory(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)
	wantSlug(t, m, "alpha")

	writeTopicIn(t, b.Root, "docs/other", "alpha.md", topicText("bug", "economy", "open", "Alpha moved"))
	writeTopicIn(t, b.Root, "docs/other", "beta.md", topicText("design", "maps", "open", "Beta"))
	writeConfig(t, b.Root, "docs/other", []string{"bug", "change", "design", "idea"}, []string{"economy", "maps"})

	pressRune(m, 'r')
	if m.board.Config.Dir != "docs/other" {
		t.Fatalf("configured dir after r = %q, want docs/other", m.board.Config.Dir)
	}
	if !strings.Contains(m.View(), "Beta") {
		t.Errorf("reload did not show the newly configured workbench:\n%s", m.View())
	}
	wantSlug(t, m, "alpha")

	pressRune(m, 'n')
	if next := readAt(t, b.Root, "docs/other/NEXT.md"); !strings.Contains(next, "[Alpha moved](alpha.md)") {
		t.Errorf("write went to the wrong workbench:\n%s", next)
	}
	if _, err := os.Stat(filepath.Join(b.Root, "docs/workbench/NEXT.md")); !os.IsNotExist(err) {
		t.Errorf("old workbench was written: err=%v", err)
	}
}

func TestReloadInvalidConfiguration(t *testing.T) {
	t.Run("explicit reload keeps the prior data", func(t *testing.T) {
		b := loadActionBoard(t, map[string]string{
			"alpha.md": topicText("bug", "economy", "open", "Alpha"),
		}, "")
		m := newActionModel(t, b)
		wantSlug(t, m, "alpha")

		writeTopic(t, b.Root, "gamma.md", topicText("design", "maps", "open", "Gamma"))
		if err := os.WriteFile(filepath.Join(b.Root, config.FileName), []byte("unknown = true\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		pressRune(m, 'r')

		for _, want := range []string{"reload failed", "unknown key"} {
			if !strings.Contains(m.status, want) {
				t.Errorf("status = %q, want it to contain %q", m.status, want)
			}
		}
		if strings.Contains(m.View(), "Gamma") {
			t.Errorf("explicit reload published the disk under an invalid config:\n%s", m.View())
		}
		wantSlug(t, m, "alpha")
		if !contains(m.board.Config.Kinds, "idea") || !contains(m.board.Config.Areas, "maps") {
			t.Errorf("configuration after a failed reload = %+v, want the prior one", m.board.Config)
		}
	})

	t.Run("saved change publishes the disk under the prior config", func(t *testing.T) {
		b := loadActionBoard(t, map[string]string{
			"alpha.md": topicText("bug", "economy", "open", "Alpha"),
		}, "")
		m := newActionModel(t, b)
		wantSlug(t, m, "alpha")

		if err := os.WriteFile(filepath.Join(b.Root, config.FileName), []byte("unknown = true\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}
		pressRune(m, 's')

		if got := readAt(t, b.Root, "docs/workbench/alpha.md"); !strings.Contains(got, "state: settled\n") {
			t.Fatalf("saved state on disk =\n%s", got)
		}
		e, ok := m.selectedEntry()
		if !ok || e.Slug != "alpha" || e.State != "settled" {
			t.Errorf("selected entry = %+v, %v; want the saved alpha state", e, ok)
		}
		for _, want := range []string{"change saved", "reload failed", "unknown key"} {
			if !strings.Contains(m.status, want) {
				t.Errorf("status = %q, want it to contain %q", m.status, want)
			}
		}
		if !contains(m.board.Config.Kinds, "idea") || !contains(m.board.Config.Areas, "maps") {
			t.Errorf("configuration after a failed reload = %+v, want the prior one", m.board.Config)
		}
	})
}

func TestEditorKeyReloads(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "open", "Alpha"),
	}, "")
	m := newActionModel(t, b)

	script := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf 'appended by editor\\n' >> \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", script)
	m.execProcess = func(c *exec.Cmd, fn tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return fn(c.Run()) }
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatalf("e returned no command")
	}
	m.Update(cmd())

	if got := readAt(t, b.Root, "docs/workbench/alpha.md"); !strings.Contains(got, "appended by editor") {
		t.Errorf("editor did not append:\n%s", got)
	}
	if m.status != "" {
		t.Errorf("status = %q, want empty after a successful edit", m.status)
	}
	if got := string(m.bodies["alpha"]); !strings.Contains(got, "appended by editor") {
		t.Errorf("bodies[alpha] = %q, want the reloaded edit", got)
	}
	if m.previewSlug != "alpha" {
		t.Errorf("preview slug = %q, want alpha", m.previewSlug)
	}
	if got := plainView(m.preview.View()); !strings.Contains(got, "appended by editor") {
		t.Errorf("preview does not show the reloaded edit:\n%s", got)
	}
}

// failingLocator is a plan lookup that always fails, like a git corruption
// after the files themselves have loaded.
type failingLocator struct{ err error }

func (l failingLocator) Find(context.Context, ...string) (map[string][]string, error) {
	return nil, l.err
}

// locatorFunc lets a test change a later reload's answer.
type locatorFunc func(context.Context, ...string) (map[string][]string, error)

func (f locatorFunc) Find(ctx context.Context, paths ...string) (map[string][]string, error) {
	return f(ctx, paths...)
}

func TestSavedChangeSurvivesLocatorFailure(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicWithBody("bug", "economy", "open", "Alpha", "the old body phrase\n"),
		"beta.md":  "---\nkind: design\narea: maps\nstate: planned\nplan: docs/plans/B.md\n---\n\n# Beta\n",
	}, "")
	m := New(Options{Board: b, Locator: failingLocator{err: errors.New("git boom")}})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	wantSlug(t, m, "alpha")

	writeTopic(t, b.Root, "alpha.md", topicWithBody("bug", "economy", "open", "Alpha", "the fresh kiwi phrase\n"))
	pressRune(m, 's')

	disk := readAt(t, b.Root, "docs/workbench/alpha.md")
	if !strings.Contains(disk, "state: settled\n") || !strings.Contains(disk, "the fresh kiwi phrase") {
		t.Fatalf("disk after s =\n%s", disk)
	}
	e, ok := m.selectedEntry()
	if !ok || e.Slug != "alpha" {
		t.Fatalf("selected entry = %+v, %v; want alpha", e, ok)
	}
	if e.State != "settled" {
		t.Errorf("entry state = %q, want settled", e.State)
	}
	if got := string(m.bodies["alpha"]); !strings.Contains(got, "the fresh kiwi phrase") {
		t.Errorf("bodies[alpha] = %q, want the reloaded body", got)
	}
	if m.previewSlug != "alpha" {
		t.Errorf("preview slug = %q, want alpha", m.previewSlug)
	}
	if got := plainView(m.preview.View()); !strings.Contains(got, "fresh kiwi phrase") {
		t.Errorf("preview does not show the reloaded body:\n%s", got)
	}
	if !strings.Contains(m.status, "change saved") || !strings.Contains(m.status, "reload failed") || !strings.Contains(m.status, "git boom") {
		t.Errorf("status = %q, want a saved-change reload failure", m.status)
	}
	if got := m.lintSummary(); got != "lint unavailable" {
		t.Errorf("lint summary = %q, want lint unavailable", got)
	}

	pressRune(m, 's')
	if m.prompt != promptPlan {
		t.Errorf("second s prompt = %v, want plan", m.prompt)
	}
}

func TestReloadFailureInvalidatesDerivedData(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": "---\nkind: bug\narea: economy\nstate: planned\nplan: docs/plans/A.md\n---\n\n# Alpha\n",
	}, "# Next\n")
	fail := false
	loc := locatorFunc(func(_ context.Context, paths ...string) (map[string][]string, error) {
		if fail {
			return nil, errors.New("git boom")
		}
		found := make(map[string][]string, len(paths))
		for _, p := range paths {
			found[p] = []string{"raised-ground"}
		}
		return found, nil
	})
	m := New(Options{
		Board:     b,
		PlanFound: map[string][]string{"docs/plans/A.md": {"raised-ground"}},
		Locator:   loc,
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	wantSlug(t, m, "alpha")
	if got := m.lintSummary(); got != "lint clean" {
		t.Fatalf("initial lint summary = %q, want clean", got)
	}

	fail = true
	pressRune(m, 'r')

	if len(m.planFound) != 0 {
		t.Errorf("planFound = %v, want cleared after a failed plan lookup", m.planFound)
	}
	if len(m.problems) != 0 {
		t.Errorf("problems = %v, want cleared after a failed refresh", m.problems)
	}
	if got := m.lintSummary(); got != "lint unavailable" {
		t.Errorf("lint summary = %q, want lint unavailable", got)
	}
	view := plainView(m.View())
	if strings.Contains(view, "lint clean") {
		t.Errorf("view claims lint clean after a failed refresh:\n%s", view)
	}
	if !strings.Contains(view, "lint unavailable") {
		t.Errorf("view does not show lint unavailable:\n%s", view)
	}
	if strings.Contains(view, "on raised-ground") {
		t.Errorf("view still shows the stale plan location:\n%s", view)
	}
	if !strings.Contains(m.status, "reload failed") {
		t.Errorf("status = %q, want reload failed", m.status)
	}
	if e, ok := m.selectedEntry(); !ok || len(e.PlanFound) != 0 {
		t.Errorf("entry plan locations = %v, want none", e.PlanFound)
	}

	fail = false
	pressRune(m, 'r')
	if got := m.lintSummary(); got != "lint clean" {
		t.Errorf("recovered lint summary = %q, want clean", got)
	}
	e, ok := m.selectedEntry()
	if !ok || len(e.PlanFound) != 1 || e.PlanFound[0] != "raised-ground" {
		t.Errorf("recovered plan locations = %v, want raised-ground", e.PlanFound)
	}
	if view := plainView(m.View()); strings.Contains(view, "lint unavailable") {
		t.Errorf("view still shows lint unavailable after recovery:\n%s", view)
	}
}

// TestChangeRerunsLint is a change whose plan the locator cannot find: the
// reload after the write must re-run lint, mark the topic and count the error,
// and the next change must clear all three.
func TestChangeRerunsLint(t *testing.T) {
	b := loadActionBoard(t, map[string]string{
		"alpha.md": topicText("bug", "economy", "settled", "Alpha"),
	}, "# Next\n\n")
	loc := locatorFunc(func(context.Context, ...string) (map[string][]string, error) {
		return map[string][]string{}, nil
	})
	m := New(Options{Board: b, Locator: loc})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	wantSlug(t, m, "alpha")

	hasRule := func(rule string) bool {
		for _, p := range m.problems {
			if p.Rule == rule {
				return true
			}
		}
		return false
	}
	entryText := func() string {
		e, ok := m.selectedEntry()
		if !ok {
			t.Fatalf("no selected entry")
		}
		return m.topicText(e, "")
	}

	pressRune(m, 's')
	if m.prompt != promptPlan {
		t.Fatalf("prompt = %v, want plan", m.prompt)
	}
	typeText(m, "missing.md")
	pressType(m, tea.KeyEnter)

	if !hasRule("plan-gone") {
		t.Errorf("problems after planning = %v, want plan-gone", m.problems)
	}
	if !m.errPaths["docs/workbench/alpha.md"] {
		t.Errorf("error paths after planning = %v, want alpha marked", m.errPaths)
	}
	if got := entryText(); !strings.Contains(got, "!") {
		t.Errorf("row after planning = %q, want an error mark", got)
	}
	if got := m.lintSummary(); got != "1 error" {
		t.Errorf("lint summary after planning = %q, want 1 error", got)
	}

	pressRune(m, 's')
	if hasRule("plan-gone") {
		t.Errorf("problems after returning open = %v, want no plan-gone", m.problems)
	}
	if m.errPaths["docs/workbench/alpha.md"] {
		t.Errorf("error paths after returning open = %v, want alpha unmarked", m.errPaths)
	}
	if got := entryText(); strings.Contains(got, "!") {
		t.Errorf("row after returning open = %q, want no error mark", got)
	}
	if got := m.lintSummary(); got != "lint clean" {
		t.Errorf("lint summary after returning open = %q, want lint clean", got)
	}
}
