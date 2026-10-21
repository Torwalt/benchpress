// Package lint checks a loaded board against the configuration and a plan
// locator, reporting the problems of docs/DESIGN.md §6. It returns values; it
// never prints.
package lint

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/repo"
	"github.com/Torwalt/benchpress/internal/topic"
)

// Severity is a problem's weight: errors fail lint, warnings do not.
type Severity string

const (
	// Error is a problem that makes benchpress lint exit 1.
	Error Severity = "error"
	// Warning is a problem lint reports but does not fail on.
	Warning Severity = "warning"
)

// Problem is one finding of §6.
type Problem struct {
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	Message  string   `json:"message"`
}

// Check runs every rule of §6 over a loaded board and loc, returning the
// problems sorted by path, then line. loc is asked once for every plan.
func Check(ctx context.Context, b *board.Board, loc repo.Locator) ([]Problem, error) {
	c := &checker{board: b, loc: loc}
	return c.run(ctx)
}

type checker struct {
	board    *board.Board
	loc      repo.Locator
	dir      string
	plans    map[string][]string // plan path -> where found
	topics   map[string]bool     // slug -> loaded
	problems []Problem
}

func (c *checker) run(ctx context.Context) ([]Problem, error) {
	c.dir = path.Clean(c.board.Config.Dir)
	c.topics = make(map[string]bool, len(c.board.Topics))
	for _, t := range c.board.Topics {
		c.topics[t.Slug] = true
	}
	if plans := c.board.Plans(); len(plans) > 0 {
		found, err := c.loc.Find(ctx, plans...)
		if err != nil {
			return nil, err
		}
		c.plans = found
	}

	for _, t := range c.board.Topics {
		c.checkTopic(t)
	}
	c.checkNext()
	c.checkNextPresent()

	sort.SliceStable(c.problems, func(i, j int) bool {
		if c.problems[i].Path != c.problems[j].Path {
			return c.problems[i].Path < c.problems[j].Path
		}
		return c.problems[i].Line < c.problems[j].Line
	})
	return c.problems, nil
}

// checkTopic runs the file-name, structural and value rules for one topic.
func (c *checker) checkTopic(t *board.Topic) {
	if !topic.ValidSlug(t.Slug + ".md") {
		c.add(Error, t.Path, 1, "slug", fmt.Sprintf("%q is not a slug", t.Slug+".md"))
	}
	for _, issue := range t.Issues {
		c.add(Error, t.Path, issue.Line, issue.Rule, issue.Message)
	}
	c.checkValues(t)
	for _, link := range t.Parsed.Links {
		c.checkLink(t.Path, link)
	}
}

// checkValues checks the front-matter values against the configuration: the
// rules kind, area, state, plan-missing, plan-unexpected and plan-gone.
func (c *checker) checkValues(t *board.Topic) {
	cfg := c.board.Config
	if f, ok := t.Parsed.Field("kind"); ok && !inList(cfg.Kinds, f.Value) {
		c.add(Error, t.Path, f.Line, "kind", fmt.Sprintf("kind %q is not configured", f.Value))
	}
	if f, ok := t.Parsed.Field("area"); ok {
		if len(cfg.Areas) > 0 {
			if !inList(cfg.Areas, f.Value) {
				c.add(Error, t.Path, f.Line, "area", fmt.Sprintf("area %q is not configured", f.Value))
			}
		} else if !topic.ValidSlug(f.Value + ".md") {
			c.add(Error, t.Path, f.Line, "area", fmt.Sprintf("area %q is not a slug", f.Value))
		}
	}
	state, hasState := t.Parsed.Field("state")
	if hasState && !validState(state.Value) {
		c.add(Error, t.Path, state.Line, "state", fmt.Sprintf("state %q is not open, settled or planned", state.Value))
	}

	plan, hasPlan := t.Parsed.Field("plan")
	planPath := ""
	if hasPlan {
		planPath = plan.Value
	}
	switch {
	case state.Value == "planned" && planPath == "":
		c.add(Error, t.Path, state.Line, "plan-missing", "state is planned without a plan")
	case state.Value != "planned" && planPath != "":
		c.add(Error, t.Path, plan.Line, "plan-unexpected", "plan is set but the state is not planned")
	case state.Value == "planned" && len(c.plans[planPath]) == 0:
		c.add(Error, t.Path, plan.Line, "plan-gone", fmt.Sprintf("plan %q is not in the working tree or on any local branch", planPath))
	}
}

// checkNext runs the rules over NEXT.md: next-item and next-duplicate, plus the
// link rules for every link an item carries.
func (c *checker) checkNext() {
	nextPath := path.Join(c.dir, "NEXT.md")
	seen := make(map[string]bool)
	for _, item := range c.board.Next.Items {
		slug := c.board.NextSlug(item.Target)
		switch {
		case slug == "" || !c.topics[slug]:
			c.add(Error, nextPath, item.Line, "next-item", nextItemMessage(slug, item.Target))
		case seen[slug]:
			c.add(Error, nextPath, item.Line, "next-duplicate", fmt.Sprintf("topic %q appears more than once in Next", slug))
		default:
			seen[slug] = true
		}
		for _, link := range topic.LineLinks(item.Text, item.Line) {
			c.checkLink(nextPath, link)
		}
	}
}

// checkNextPresent warns when the workbench has no NEXT.md.
func (c *checker) checkNextPresent() {
	nextPath := path.Join(c.dir, "NEXT.md")
	if !c.exists(nextPath) {
		c.add(Warning, nextPath, 1, "next-absent", fmt.Sprintf("%s has no NEXT.md", c.dir))
	}
}

// checkLink reports a link to a missing *.md file in dir as topic-link, and a
// relative link to a missing file outside dir as the warning link.
func (c *checker) checkLink(file string, link topic.Link) {
	resolved := path.Clean(path.Join(c.dir, link.Target))
	withinDir := inside(c.dir, resolved)
	switch {
	case withinDir && strings.HasSuffix(link.Target, ".md"):
		if !c.exists(resolved) {
			c.add(Error, file, link.Line, "topic-link", fmt.Sprintf("link to missing topic file %q", link.Target))
		}
	case !withinDir:
		if !c.exists(resolved) {
			c.add(Warning, file, link.Line, "link", fmt.Sprintf("link to missing file %q", link.Target))
		}
	}
}

// add records one problem.
func (c *checker) add(severity Severity, p string, line int, rule, message string) {
	c.problems = append(c.problems, Problem{Path: p, Line: line, Severity: severity, Rule: rule, Message: message})
}

// exists reports whether a repository-relative path is in the working tree.
func (c *checker) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(c.board.Root, filepath.FromSlash(rel)))
	return err == nil
}

// inside reports whether resolved is dir itself or lexically below it, after
// path normalization, including when dir is ".".
func inside(dir, resolved string) bool {
	rel, err := filepath.Rel(filepath.FromSlash(dir), filepath.FromSlash(resolved))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func nextItemMessage(slug, target string) string {
	if slug == "" {
		if target == "" {
			return "Next item does not begin with a link"
		}
		return fmt.Sprintf("Next item links to %q, not a topic", target)
	}
	return fmt.Sprintf("Next item links to missing topic %q", slug)
}

func validState(s string) bool {
	switch s {
	case "open", "settled", "planned":
		return true
	}
	return false
}

func inList(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}
