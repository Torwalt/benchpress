// Package tui draws the board of docs/DESIGN.md §9 over a loaded board and the
// problems lint found. It changes files only through board's operations, and
// calls lint again after a reload.
package tui

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/lint"
	"github.com/Torwalt/benchpress/internal/repo"
)

// narrowWidth is the column count below which the preview pane is hidden.
const narrowWidth = 100

// nextGroupLabel names the Next section's nonselectable row.
const nextGroupLabel = "Next"

// groupings is the order tab cycles the list grouping.
var groupings = []string{"area", "state", "kind"}

// Options is the data the board draws itself from.
type Options struct {
	Board     *board.Board
	PlanFound map[string][]string
	Problems  []lint.Problem
	// Locator finds plans again after a change; nil skips the lookup.
	Locator repo.Locator
}

// rowKind distinguishes the rows of the flattened list.
type rowKind int

const (
	rowNext rowKind = iota
	rowGroup
	rowTopic
)

// row is one line of the list.
type row struct {
	kind   rowKind
	label  string // a group's name
	entry  board.Entry
	number int // a Next item's 1-based position
}

// group is a titled run of topics.
type group struct {
	name    string
	entries []board.Entry
}

// Model is the board's bubbletea model. A pointer receiver so Update can mutate
// it in place.
type Model struct {
	board     *board.Board
	planFound map[string][]string
	problems  []lint.Problem
	locator   repo.Locator
	errPaths  map[string]bool
	bodies    map[string][]byte
	status    string // the last failed operation's message

	input    textinput.Model
	prompt   promptKind
	newTitle string
	newKind  string

	confirming    bool
	confirmSlug   string
	lintAvailable bool

	execProcess execProcessFunc

	width, height int
	listHeight    int
	ready         bool

	grouping  string
	filter    string
	filtering bool

	rows   []row
	cursor int
	offset int

	narrow        bool
	narrowPreview bool
	showHelp      bool

	preview      viewport.Model
	previewSlug  string
	previewWidth int
}

// New builds the board model from a loaded board and its lint problems.
func New(opts Options) *Model {
	m := &Model{
		board:         opts.Board,
		planFound:     opts.PlanFound,
		problems:      opts.Problems,
		locator:       opts.Locator,
		errPaths:      errorPaths(opts.Problems),
		bodies:        topicBodies(opts.Board),
		grouping:      groupings[0],
		input:         textinput.New(),
		execProcess:   tea.ExecProcess,
		lintAvailable: true,
	}
	m.rebuild()
	return m
}

// errorPaths is the set of paths a lint error names.
func errorPaths(problems []lint.Problem) map[string]bool {
	paths := make(map[string]bool)
	for _, p := range problems {
		if p.Severity == lint.Error {
			paths[p.Path] = true
		}
	}
	return paths
}

// topicBodies maps every loaded topic's slug to its body.
func topicBodies(b *board.Board) map[string][]byte {
	bodies := make(map[string][]byte)
	if b == nil {
		return bodies
	}
	for _, t := range b.Topics {
		bodies[t.Slug] = t.Body()
	}
	return bodies
}

// Init starts the model with nothing to do.
func (m *Model) Init() tea.Cmd { return nil }

// Update handles a message. Window sizes and read-only keys are all it knows.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		m.syncPreview()
		m.adjustOffset()
	case tea.KeyMsg:
		return m.key(msg)
	case editorClosedMsg:
		m.status = ""
		if msg.err != nil {
			m.status = msg.err.Error()
		}
		if err := m.reload(false); err != nil {
			m.status = "reload failed: " + err.Error()
		}
		return m, nil
	}
	return m, nil
}

// key handles one key press, dispatching on what the board is doing.
func (m *Model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if m.quits(msg) {
		return m, tea.Quit
	}
	if m.showHelp {
		switch msg.String() {
		case "?", "esc":
			m.showHelp = false
		}
		return m, nil
	}
	if m.prompt != promptNone {
		return m.promptKey(msg)
	}
	if m.confirming {
		return m.confirmKey(msg)
	}
	if m.filtering {
		return m.filterKey(msg)
	}
	switch msg.String() {
	case "?":
		m.showHelp = true
	case "/":
		m.filtering = true
	case "esc":
		m.clearFilter()
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		m.firstRow()
	case "G":
		m.lastRow()
	case "tab":
		m.cycleGrouping()
	case "ctrl+d":
		m.preview.HalfPageDown()
	case "ctrl+u":
		m.preview.HalfPageUp()
	case "s":
		return m, m.cycleState()
	case "n":
		m.toggleNext()
		return m, nil
	case "J":
		m.moveNext(1)
		return m, nil
	case "K":
		m.moveNext(-1)
		return m, nil
	case "a":
		return m, m.startNew()
	case "d":
		m.startDelete()
		return m, nil
	case "r":
		if err := m.reload(false); err != nil {
			m.status = "reload failed: " + err.Error()
		}
		return m, nil
	case "e":
		return m, m.editSelected()
	case "enter":
		if m.narrow {
			m.narrowPreview = !m.narrowPreview
			m.syncPreview()
		} else {
			return m, m.editSelected()
		}
	}
	m.afterMove()
	return m, nil
}

// quits reports whether the key ends the board instead of reaching the active
// mode. Ctrl+C always quits; q quits everywhere except the text modes, where it
// is text.
func (m *Model) quits(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+c":
		return true
	case "q":
		return m.prompt == promptNone && !m.filtering
	}
	return false
}

// filterKey handles a key while the filter is being typed.
func (m *Model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.clearFilter()
		m.filtering = false
	case "enter":
		m.filtering = false
	case "backspace":
		if m.filter != "" {
			_, size := utf8.DecodeLastRuneInString(m.filter)
			m.filter = m.filter[:len(m.filter)-size]
		}
		m.rebuild()
		m.afterMove()
	default:
		if len(msg.Runes) > 0 {
			m.filter += string(msg.Runes)
			m.rebuild()
			m.afterMove()
		}
	}
	return m, nil
}

// afterMove keeps the preview and the scroll in step with the selection.
func (m *Model) afterMove() {
	m.adjustOffset()
	m.syncPreview()
}

// clearFilter drops the filter and rebuilds the list.
func (m *Model) clearFilter() {
	if m.filter == "" {
		return
	}
	m.filter = ""
	m.rebuild()
	m.afterMove()
}

// rebuild flattens the filtered board into rows: the Next section first, then
// the groups of the chosen grouping.
func (m *Model) rebuild() {
	slug := m.selectedSlug()
	m.rows = m.rows[:0]
	if m.board == nil {
		m.cursor = 0
		return
	}
	nextSlugs, entries := m.board.Listing(board.Filter{}, m.planFound)
	entries = m.applyFilter(entries)
	if m.filter != "" {
		present := make(map[string]bool, len(entries))
		for _, e := range entries {
			present[e.Slug] = true
		}
		kept := make([]string, 0, len(nextSlugs))
		for _, s := range nextSlugs {
			if present[s] {
				kept = append(kept, s)
			}
		}
		nextSlugs = kept
	}

	bySlug := make(map[string]board.Entry, len(entries))
	for _, e := range entries {
		bySlug[e.Slug] = e
	}
	inNext := make(map[string]bool, len(nextSlugs))
	for _, s := range nextSlugs {
		inNext[s] = true
	}
	if len(nextSlugs) > 0 {
		m.rows = append(m.rows, row{kind: rowGroup, label: nextGroupLabel})
	}
	for i, s := range nextSlugs {
		m.rows = append(m.rows, row{kind: rowNext, entry: bySlug[s], number: i + 1})
	}
	var rest []board.Entry
	for _, e := range entries {
		if !inNext[e.Slug] {
			rest = append(rest, e)
		}
	}
	for _, g := range m.groupEntries(rest) {
		m.rows = append(m.rows, row{kind: rowGroup, label: g.name})
		for _, e := range g.entries {
			m.rows = append(m.rows, row{kind: rowTopic, entry: e})
		}
	}
	m.restoreCursor(slug)
}

// applyFilter keeps the entries whose title, slug, kind or area contain the
// filter text, case-insensitively.
func (m *Model) applyFilter(entries []board.Entry) []board.Entry {
	if m.filter == "" {
		return entries
	}
	q := strings.ToLower(m.filter)
	out := make([]board.Entry, 0, len(entries))
	for _, e := range entries {
		if entryMatches(e, q) {
			out = append(out, e)
		}
	}
	return out
}

func entryMatches(e board.Entry, q string) bool {
	for _, field := range []string{e.Title, e.Slug, e.Kind, e.Area} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	return false
}

// groupEntries splits the non-Next topics into groups for the current grouping,
// ordered as the configuration orders them (states by their fixed order).
func (m *Model) groupEntries(entries []board.Entry) []group {
	order := m.groupOrder()
	index := make(map[string]int)
	var groups []group
	for _, e := range entries {
		name := m.groupKey(e)
		i, ok := index[name]
		if !ok {
			i = len(groups)
			index[name] = i
			groups = append(groups, group{name: name})
		}
		groups[i].entries = append(groups[i].entries, e)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		ri, si := groupRank(order, groups[i].name)
		rj, sj := groupRank(order, groups[j].name)
		if ri != rj {
			return ri < rj
		}
		return si < sj
	})
	for i := range groups {
		if groups[i].name == "" {
			groups[i].name = "(none)"
		}
	}
	return groups
}

// groupKey is the field the current grouping groups by.
func (m *Model) groupKey(e board.Entry) string {
	switch m.grouping {
	case "state":
		return e.State
	case "kind":
		return e.Kind
	default:
		return e.Area
	}
}

// groupOrder is the configured order of the current grouping's values.
func (m *Model) groupOrder() []string {
	if m.board == nil {
		return nil
	}
	switch m.grouping {
	case "state":
		return []string{"open", "settled", "planned"}
	case "kind":
		return m.board.Config.Kinds
	default:
		return m.board.Config.Areas
	}
}

// groupRank ranks a group name: configured values in order, then other non-empty
// names alphabetically, then no name at all.
func groupRank(order []string, name string) (int, string) {
	if name == "" {
		return len(order) + 1, ""
	}
	for i, o := range order {
		if o == name {
			return i, ""
		}
	}
	return len(order), name
}

// restoreCursor puts the cursor on slug's row, or on the first topic row.
func (m *Model) restoreCursor(slug string) {
	m.cursor = -1
	for i, r := range m.rows {
		if !selectable(r.kind) {
			continue
		}
		if m.cursor < 0 {
			m.cursor = i
		}
		if r.entry.Slug == slug {
			m.cursor = i
			return
		}
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// selectable reports whether a row is a topic the cursor can rest on. Group
// headers are not.
func selectable(k rowKind) bool { return k != rowGroup }

// move steps the cursor over delta topic rows, skipping group headers.
func (m *Model) move(delta int) {
	for i := m.cursor + delta; i >= 0 && i < len(m.rows); i += delta {
		if selectable(m.rows[i].kind) {
			m.cursor = i
			return
		}
	}
}

// firstRow selects the first topic.
func (m *Model) firstRow() {
	for i := range m.rows {
		if selectable(m.rows[i].kind) {
			m.cursor = i
			return
		}
	}
}

// lastRow selects the last topic.
func (m *Model) lastRow() {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if selectable(m.rows[i].kind) {
			m.cursor = i
			return
		}
	}
}

// cycleGrouping advances area → state → kind → area.
func (m *Model) cycleGrouping() {
	for i, g := range groupings {
		if g == m.grouping {
			m.grouping = groupings[(i+1)%len(groupings)]
			break
		}
	}
	m.rebuild()
}

// selectedEntry is the entry under the cursor, when it is a topic row.
func (m *Model) selectedEntry() (board.Entry, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) || !selectable(m.rows[m.cursor].kind) {
		return board.Entry{}, false
	}
	return m.rows[m.cursor].entry, true
}

// selectedSlug is the selected topic's slug, or "".
func (m *Model) selectedSlug() string {
	e, ok := m.selectedEntry()
	if !ok {
		return ""
	}
	return e.Slug
}

// layout sizes the panes from the terminal size.
func (m *Model) layout() {
	m.narrow = m.width < narrowWidth
	m.listHeight = m.height - 1
	if m.listHeight < 1 {
		m.listHeight = 1
	}
	previewWidth := m.width
	if !m.narrow {
		previewWidth = m.width - m.listWidth() - 2
	}
	if previewWidth < 1 {
		previewWidth = 1
	}
	m.preview.Width = previewWidth
	m.preview.Height = m.listHeight
}

// listWidth is the width of the list pane.
func (m *Model) listWidth() int {
	w := m.width * 2 / 5
	if w < 20 {
		w = 20
	}
	if w > m.width-10 {
		w = m.width - 10
	}
	if w < 1 {
		w = 1
	}
	return w
}

// adjustOffset scrolls the list so the cursor is visible.
func (m *Model) adjustOffset() {
	if m.listHeight <= 0 || m.cursor < 0 {
		m.offset = 0
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.listHeight {
		m.offset = m.cursor - m.listHeight + 1
	}
	if max := len(m.rows) - m.listHeight; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// syncPreview renders the selected topic's body into the preview viewport when
// the selection or the pane's width has changed.
func (m *Model) syncPreview() {
	if !m.ready || (m.narrow && !m.narrowPreview) {
		return
	}
	e, ok := m.selectedEntry()
	if !ok {
		m.preview.SetContent("")
		m.previewSlug = ""
		return
	}
	width := m.preview.Width
	if width < 1 {
		width = 1
	}
	if e.Slug == m.previewSlug && width == m.previewWidth {
		return
	}
	m.preview.SetContent(renderMarkdown(string(m.bodies[e.Slug]), width))
	m.preview.GotoTop()
	m.previewSlug = e.Slug
	m.previewWidth = width
}

// renderMarkdown renders Markdown for a terminal of the given width, falling
// back to the source when glamour will not.
func renderMarkdown(src string, width int) string {
	renderer, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(width))
	if err != nil {
		return src
	}
	out, err := renderer.Render(src)
	if err != nil {
		return src
	}
	return out
}
