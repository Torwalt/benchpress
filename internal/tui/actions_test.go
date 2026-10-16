package tui

import (
	"context"
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
}
