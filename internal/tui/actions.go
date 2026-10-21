// The board's file-changing keys of docs/DESIGN.md §9. Each runs through a
// board operation, then reloads and re-runs lint; nothing edits a file here.
package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/lint"
)

// promptKind is the file-changing flow waiting for typed input.
type promptKind int

const (
	promptNone promptKind = iota
	promptPlan
	promptNewTitle
	promptNewKind
	promptNewArea
)

// editorClosedMsg reports that the editor exited.
type editorClosedMsg struct{ err error }

// execProcessFunc runs an interactive command. It is tea.ExecProcess by
// default; a field so tests can run one synchronously.
type execProcessFunc func(*exec.Cmd, tea.ExecCallback) tea.Cmd

// planLocator answers lint from a plan lookup already made, so one reload asks
// the locator once.
type planLocator map[string][]string

func (l planLocator) Find(context.Context, ...string) (map[string][]string, error) {
	return l, nil
}

// editorCommand opens path in $VISUAL, else $EDITOR, else vi.
func editorCommand(path string) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		fields = []string{"vi"}
	}
	return exec.Command(fields[0], append(fields[1:], path)...)
}

// change runs one board operation. On success it reloads and re-lints; on
// failure it shows the error and changes nothing.
func (m *Model) change(op func() error) {
	m.status = ""
	if err := op(); err != nil {
		m.status = err.Error()
		return
	}
	if err := m.reload(); err != nil {
		m.status = err.Error()
	}
}

// reload re-reads the workbench, re-runs lint and rebuilds the list, keeping the
// selected topic when it still exists.
func (m *Model) reload() error {
	if m.board == nil {
		return nil
	}
	b, err := board.Load(m.board.Root, m.board.Config)
	if err != nil {
		return err
	}
	found, err := m.findPlans(b)
	if err != nil {
		return err
	}
	problems, err := lint.Check(context.Background(), b, b.Exists, planLocator(found))
	if err != nil {
		return err
	}
	m.board = b
	m.planFound = found
	m.problems = problems
	m.errPaths = errorPaths(problems)
	m.bodies = topicBodies(b)
	m.previewSlug = "" // force the preview to re-render the reloaded body
	m.rebuild()
	m.afterMove()
	return nil
}

// findPlans locates every planned topic's plan through the board's locator.
func (m *Model) findPlans(b *board.Board) (map[string][]string, error) {
	plans := b.Plans()
	if len(plans) == 0 || m.locator == nil {
		return nil, nil
	}
	return m.locator.Find(context.Background(), plans...)
}

// cycleState advances the selected topic open → settled → planned → open.
// Reaching planned asks for the plan path instead of writing at once.
func (m *Model) cycleState() tea.Cmd {
	e, ok := m.selectedEntry()
	if !ok {
		return nil
	}
	switch e.State {
	case "planned":
		m.change(func() error { return m.board.SetState(e.Slug, "open", "") })
	case "settled":
		m.startPlanPrompt()
		return m.input.Focus()
	default:
		m.change(func() error { return m.board.SetState(e.Slug, "settled", "") })
	}
	return nil
}

// toggleNext adds the selected topic to Next, or removes it when there.
func (m *Model) toggleNext() {
	e, ok := m.selectedEntry()
	if !ok {
		return
	}
	if m.board.NextIndex(e.Slug) >= 0 {
		m.change(func() error { return m.board.NextRemove(e.Slug) })
		return
	}
	m.change(func() error { return m.board.NextAdd(e.Slug, 0) })
}

// moveNext moves the selected Next item down (delta 1) or up (delta -1).
func (m *Model) moveNext(delta int) {
	e, ok := m.selectedEntry()
	if !ok {
		return
	}
	at := m.board.NextIndex(e.Slug) + 1 + delta
	m.change(func() error { return m.board.NextMove(e.Slug, at) })
}

// startNew begins the add-topic prompts.
func (m *Model) startNew() tea.Cmd {
	m.prompt = promptNewTitle
	m.newTitle, m.newKind = "", ""
	m.setInput("title: ", "")
	return m.input.Focus()
}

// startPlanPrompt begins the plan-path prompt, prefilled with docs/plans/.
func (m *Model) startPlanPrompt() {
	m.prompt = promptPlan
	m.setInput("plan: ", "docs/plans/")
}

// startDelete asks for confirmation before deleting the selected topic.
func (m *Model) startDelete() {
	if e, ok := m.selectedEntry(); ok {
		m.confirming = true
		m.confirmSlug = e.Slug
	}
}

// editSelected opens the selected topic in the editor through tea.ExecProcess.
func (m *Model) editSelected() tea.Cmd {
	e, ok := m.selectedEntry()
	if !ok {
		return nil
	}
	path := filepath.Join(m.board.Root, filepath.FromSlash(e.Path))
	return m.execProcess(editorCommand(path), func(err error) tea.Msg {
		return editorClosedMsg{err: err}
	})
}

// promptKey handles a key while a prompt is being answered.
func (m *Model) promptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.resetPrompt()
		return m, nil
	case "enter":
		return m, m.submitPrompt()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// confirmKey handles the delete confirmation: only y deletes.
func (m *Model) confirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.confirming = false
	if msg.String() != "y" {
		return m, nil
	}
	slug := m.confirmSlug
	m.change(func() error {
		_, err := m.board.Remove(slug)
		return err
	})
	return m, nil
}

// submitPrompt advances or finishes the active prompt.
func (m *Model) submitPrompt() tea.Cmd {
	switch m.prompt {
	case promptPlan:
		slug := m.selectedSlug()
		plan := strings.TrimSpace(m.input.Value())
		m.resetPrompt()
		if slug == "" {
			return nil
		}
		m.change(func() error { return m.board.SetState(slug, "planned", plan) })
	case promptNewTitle:
		title := strings.TrimSpace(m.input.Value())
		if title == "" {
			m.status = "a title is required"
			return nil
		}
		m.newTitle = title
		m.prompt = promptNewKind
		m.setInput("kind: ", "")
		return m.input.Focus()
	case promptNewKind:
		kind := strings.TrimSpace(m.input.Value())
		if kind == "" {
			m.status = "a kind is required"
			return nil
		}
		m.newKind = kind
		m.prompt = promptNewArea
		m.setInput("area: ", "")
		return m.input.Focus()
	case promptNewArea:
		area := strings.TrimSpace(m.input.Value())
		title, kind := m.newTitle, m.newKind
		m.resetPrompt()
		m.change(func() error {
			_, err := m.board.New(title, kind, area, false)
			return err
		})
	}
	return nil
}

// resetPrompt clears the prompt and its input without changing anything.
func (m *Model) resetPrompt() {
	m.prompt = promptNone
	m.newTitle, m.newKind = "", ""
	m.input.Blur()
	m.input.Reset()
}

// setInput points the text input at a new prompt and value.
func (m *Model) setInput(prompt, value string) {
	m.input.Prompt = prompt
	m.input.SetValue(value)
	m.input.CursorEnd()
}
