// Package board loads a workbench directory: its topics and Next, the order ls
// shows them in, and the §7 operations that write topic and Next files.
package board

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Torwalt/benchpress/internal/config"
	"github.com/Torwalt/benchpress/internal/next"
	"github.com/Torwalt/benchpress/internal/topic"
)

// Topic is one loaded topic file.
type Topic struct {
	Slug   string // the file name without .md
	Path   string // repository-relative, slash-separated
	Parsed topic.Topic
	Issues []topic.Issue // structural issues from topic.Parse
}

// Kind is the topic's kind, or "" when it is missing.
func (t *Topic) Kind() string { return t.field("kind") }

// Area is the topic's area, or "" when it is missing.
func (t *Topic) Area() string { return t.field("area") }

// State is the topic's state, or "" when it is missing.
func (t *Topic) State() string { return t.field("state") }

// Plan is the topic's plan path, or "" when it is missing.
func (t *Topic) Plan() string { return t.field("plan") }

// Title is the topic's level-one heading, or "" when it has none.
func (t *Topic) Title() string { return t.Parsed.Title }

// Body is the topic's Markdown after its front matter.
func (t *Topic) Body() []byte { return t.Parsed.Body }

// Broken reports a topic with a structural issue: it did not parse cleanly.
func (t *Topic) Broken() bool { return len(t.Issues) > 0 }

// FirstIssue is the topic's first structural issue, in line order.
func (t *Topic) FirstIssue() (topic.Issue, bool) {
	if len(t.Issues) == 0 {
		return topic.Issue{}, false
	}
	return t.Issues[0], true
}

func (t *Topic) field(key string) string {
	f, ok := t.Parsed.Field(key)
	if !ok {
		return ""
	}
	return f.Value
}

// Board is a loaded workbench: every topic in the configured directory and the
// parsed NEXT.md.
type Board struct {
	Root   string
	Config *config.Config
	Topics []*Topic
	Next   next.Next

	// writeNextFile is writeAtomic except in tests, which inject a failure to
	// exercise the second step of a multi-file operation.
	writeNextFile func(name string, data []byte, mode fs.FileMode) error
}

// Load reads every topic file and NEXT.md from root's configured directory.
// A missing directory yields an empty board; a missing NEXT.md an empty Next.
func Load(root string, cfg *config.Config) (*Board, error) {
	b := &Board{Root: root, Config: cfg, Next: next.New()}
	dir := filepath.Join(root, filepath.FromSlash(cfg.Dir))

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return b, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || name == "NEXT.md" || name == "README.md" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		parsed, issues := topic.Parse(src)
		b.Topics = append(b.Topics, &Topic{
			Slug:   strings.TrimSuffix(name, ".md"),
			Path:   filepath.ToSlash(filepath.Join(cfg.Dir, name)),
			Parsed: parsed,
			Issues: issues,
		})
	}
	sort.Slice(b.Topics, func(i, j int) bool { return b.Topics[i].Slug < b.Topics[j].Slug })

	nextSrc, err := os.ReadFile(filepath.Join(dir, "NEXT.md"))
	switch {
	case err == nil:
		b.Next = next.Parse(nextSrc)
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	return b, nil
}

// Filter narrows a listing to the given states, kinds and areas. An empty
// slice does not filter.
type Filter struct {
	States []string
	Kinds  []string
	Areas  []string
}

func (f Filter) match(t *Topic) bool {
	return inList(f.States, t.State()) && inList(f.Kinds, t.Kind()) && inList(f.Areas, t.Area())
}

func inList(list []string, value string) bool {
	if len(list) == 0 {
		return true
	}
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// Entry is one topic as ls lists it.
type Entry struct {
	Slug      string
	Path      string
	Title     string
	Kind      string
	Area      string
	State     string
	Plan      string
	PlanFound []string // where the plan was found, for a planned topic
	Issue     *topic.Issue
}

// Broken reports a topic listed by its parse error rather than its fields.
func (e Entry) Broken() bool { return e.Issue != nil }

// Listing returns the ls view: the slugs of the Next items shown, in order,
// then every shown topic in display order — Next first, then other topics by
// area (configured order, then alphabetical), state (open, settled, planned)
// and title. planFound maps each plan path to the places it was found.
func (b *Board) Listing(f Filter, planFound map[string][]string) ([]string, []Entry) {
	bySlug := make(map[string]*Topic, len(b.Topics))
	for _, t := range b.Topics {
		bySlug[t.Slug] = t
	}

	nextSlugs := []string{}
	inNext := make(map[string]bool)
	for _, item := range b.Next.Items {
		slug := b.NextSlug(item.Target)
		if slug == "" || inNext[slug] {
			continue
		}
		t, ok := bySlug[slug]
		if !ok || !f.match(t) {
			continue
		}
		inNext[slug] = true
		nextSlugs = append(nextSlugs, slug)
	}

	var rest []*Topic
	for _, t := range b.Topics {
		if inNext[t.Slug] || !f.match(t) {
			continue
		}
		rest = append(rest, t)
	}
	sort.SliceStable(rest, func(i, j int) bool { return b.less(rest[i], rest[j]) })

	entries := make([]Entry, 0, len(nextSlugs)+len(rest))
	for _, slug := range nextSlugs {
		entries = append(entries, b.entry(bySlug[slug], planFound))
	}
	for _, t := range rest {
		entries = append(entries, b.entry(t, planFound))
	}
	return nextSlugs, entries
}

// Exists reports whether a repository-relative path is in the working tree.
// A stat error counts as absence.
func (b *Board) Exists(rel string) bool {
	_, err := os.Stat(filepath.Join(b.Root, filepath.FromSlash(rel)))
	return err == nil
}

// Plans returns the distinct plan paths of the planned topics.
func (b *Board) Plans() []string {
	seen := make(map[string]bool)
	var out []string
	for _, t := range b.Topics {
		if t.State() != "planned" {
			continue
		}
		p := t.Plan()
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (b *Board) entry(t *Topic, planFound map[string][]string) Entry {
	e := Entry{
		Slug:  t.Slug,
		Path:  t.Path,
		Title: t.Title(),
		Kind:  t.Kind(),
		Area:  t.Area(),
		State: t.State(),
		Plan:  t.Plan(),
	}
	if issue, ok := t.FirstIssue(); ok {
		e.Issue = &issue
	}
	if e.State == "planned" && e.Plan != "" {
		e.PlanFound = planFound[e.Plan]
	}
	return e
}

// less orders two topics by area, state, title and slug, as §7 says.
func (b *Board) less(a, c *Topic) bool {
	ra, na := b.areaKey(a.Area())
	rc, nc := b.areaKey(c.Area())
	if ra != rc {
		return ra < rc
	}
	if na != nc {
		return na < nc
	}
	if sa, sc := stateRank(a.State()), stateRank(c.State()); sa != sc {
		return sa < sc
	}
	if a.Title() != c.Title() {
		return a.Title() < c.Title()
	}
	return a.Slug < c.Slug
}

// areaKey ranks a topic's area: configured areas in order, then other
// non-empty areas alphabetically, then no area at all.
func (b *Board) areaKey(area string) (int, string) {
	if area == "" {
		return len(b.Config.Areas) + 1, ""
	}
	for i, a := range b.Config.Areas {
		if a == area {
			return i, area
		}
	}
	return len(b.Config.Areas), area
}

func stateRank(state string) int {
	switch state {
	case "open":
		return 0
	case "settled":
		return 1
	case "planned":
		return 2
	}
	return 3
}

// NextSlug returns the slug a Next target names when it is a relative path
// that normalizes to a topic file directly in the configured directory, or ""
// for anything else.
func (b *Board) NextSlug(target string) string {
	if target == "" || path.IsAbs(target) || !strings.HasSuffix(target, ".md") {
		return ""
	}
	dir := path.Clean(b.Config.Dir)
	resolved := path.Clean(path.Join(dir, target))
	if path.Dir(resolved) != dir {
		return ""
	}
	return strings.TrimSuffix(path.Base(resolved), ".md")
}
