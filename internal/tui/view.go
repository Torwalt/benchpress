package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/lint"
)

// worktree is repo.Worktree's value; a planned topic whose plan is here shows no
// branch.
const worktree = "worktree"

var (
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	groupStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	headingStyle  = lipgloss.NewStyle().Bold(true)
)

// View draws the board: list, preview, status line.
func (m *Model) View() string {
	if !m.ready {
		return ""
	}
	if m.showHelp {
		return m.helpView()
	}
	status := m.statusView()
	if m.narrow {
		if m.narrowPreview {
			return clampLines(m.preview.View(), m.height-1) + "\n" + status
		}
		return clampLines(m.listView(), m.height-1) + "\n" + status
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.listView(), "  ", m.preview.View())
	return clampLines(body, m.height-1) + "\n" + status
}

// listView renders the visible window of the list, padded to its height.
func (m *Model) listView() string {
	if m.listHeight <= 0 {
		return ""
	}
	lines := make([]string, 0, m.listHeight)
	for i := m.offset; i < len(m.rows) && len(lines) < m.listHeight; i++ {
		lines = append(lines, m.rowView(i))
	}
	for len(lines) < m.listHeight {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// rowView renders one row, truncated to the list pane and marked when selected.
func (m *Model) rowView(i int) string {
	r := m.rows[i]
	selected := i == m.cursor
	var text string
	switch r.kind {
	case rowGroup:
		text = r.label
	case rowNext:
		text = m.topicText(r.entry, fmt.Sprintf("%d. ", r.number))
	default:
		text = m.topicText(r.entry, "")
	}
	width := m.width
	if !m.narrow {
		width = m.listWidth()
	}
	text = truncate(text, width-2)
	switch {
	case r.kind == rowGroup:
		return groupStyle.Render(text)
	case selected:
		return selectedStyle.Render(text)
	default:
		return "  " + text
	}
}

// topicText is a topic row's content: the state mark, the kind, the title and,
// when a planned topic's plan is not in the working tree, the branch it is on.
func (m *Model) topicText(e board.Entry, prefix string) string {
	if e.Broken() {
		return prefix + e.Slug + "  " + e.Issue.Message
	}
	text := prefix + stateMark(e.State) + " " + e.Kind + "  " + e.Title
	if e.State == "planned" && len(e.PlanFound) > 0 && !contains(e.PlanFound, worktree) {
		text += "  on " + e.PlanFound[0]
	}
	if m.errPaths[e.Path] {
		text += "  !"
	}
	return text
}

// stateMark is §9's mark for a state.
func stateMark(state string) string {
	switch state {
	case "open":
		return "○"
	case "settled":
		return "◆"
	case "planned":
		return "▶"
	}
	return "·"
}

// statusView is the bottom line: a prompt, the delete confirmation, or the
// lint summary, the active filter and grouping, and a one-line hint. A failed
// operation's error leads.
func (m *Model) statusView() string {
	switch {
	case m.prompt != promptNone:
		text := m.input.View()
		if m.status != "" {
			text = m.status + "  " + text
		}
		return statusStyle.Render(truncate(text, m.width))
	case m.confirming:
		return statusStyle.Render(truncate("delete "+m.confirmSlug+"? y/N", m.width))
	}
	var parts []string
	if m.status != "" {
		parts = append(parts, m.status)
	}
	parts = append(parts, m.lintSummary())
	if m.filter != "" {
		parts = append(parts, "filter: "+m.filter)
	}
	parts = append(parts, "group: "+m.grouping)
	parts = append(parts, "j/k move  / filter  tab group  ? help  q quit")
	return statusStyle.Render(truncate(strings.Join(parts, "  ·  "), m.width))
}

// lintSummary is the counts of §6's problems, or "lint clean"; it is
// "lint unavailable" until a reload completes both plan lookup and lint.
func (m *Model) lintSummary() string {
	if !m.lintAvailable {
		return "lint unavailable"
	}
	var errs, warns int
	for _, p := range m.problems {
		if p.Severity == lint.Error {
			errs++
		} else {
			warns++
		}
	}
	switch {
	case errs == 0 && warns == 0:
		return "lint clean"
	case warns == 0:
		return count(errs, "error")
	case errs == 0:
		return count(warns, "warning")
	default:
		return count(errs, "error") + ", " + count(warns, "warning")
	}
}

// helpView is the full key list of §9.
func (m *Model) helpView() string {
	lines := []string{
		headingStyle.Render("benchpress board"),
		"",
		"  j k ↓ ↑        move the selection",
		"  g G            first and last row",
		"  ctrl+d ctrl+u  scroll the preview",
		"  /              filter by title, slug, kind, area; esc clears",
		"  tab            cycle the grouping: area, state, kind",
		"  s              cycle the state; planned asks for the plan path",
		"  n              add the topic to Next, or remove it",
		"  J K            move a Next item down or up",
		"  e enter        open the topic in the editor",
		"  a              add a topic",
		"  d              delete the topic after a y confirmation",
		"  r              reload from disk",
		"  ?              this help",
		"  q ctrl+c       quit",
		"",
		helpStyle.Render("  press ? or esc to close"),
	}
	return strings.Join(lines, "\n")
}

// count renders "N error(s)".
func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// contains reports whether list holds value.
func contains(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// truncate cuts s to width display columns, counting ANSI-aware display width.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

// clampLines keeps at most n lines of s.
func clampLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}
