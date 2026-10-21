package lint

import (
	"context"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/config"
	"github.com/Torwalt/benchpress/internal/next"
	"github.com/Torwalt/benchpress/internal/repo"
	"github.com/Torwalt/benchpress/internal/topic"
)

// fakeLocator answers Find from a map, so lint's tests need no git.
type fakeLocator map[string][]string

func (f fakeLocator) Find(_ context.Context, paths ...string) (map[string][]string, error) {
	found := make(map[string][]string, len(paths))
	for _, p := range paths {
		found[p] = f[p]
	}
	return found, nil
}

var _ repo.Locator = fakeLocator(nil)

func defaultConfig() *config.Config {
	return &config.Config{
		Dir:   "docs/workbench",
		Kinds: []string{"bug", "change", "design", "idea"},
		Areas: []string{"economy", "maps"},
	}
}

// boardFrom builds a board in memory and the repository-relative paths that
// exist for it.
func boardFrom(t *testing.T, cfg *config.Config, topics map[string]string, nextSrc string) (*board.Board, map[string]bool) {
	t.Helper()
	dir := path.Clean(cfg.Dir)
	b := &board.Board{Root: ".", Config: cfg, Next: next.New()}
	exists := make(map[string]bool)
	for name, body := range topics {
		parsed, issues := topic.Parse([]byte(body))
		b.Topics = append(b.Topics, &board.Topic{
			Slug:   strings.TrimSuffix(name, ".md"),
			Path:   path.Join(dir, name),
			Parsed: parsed,
			Issues: issues,
		})
		exists[path.Join(dir, name)] = true
	}
	sort.Slice(b.Topics, func(i, j int) bool { return b.Topics[i].Slug < b.Topics[j].Slug })
	if nextSrc != "" {
		b.Next = next.Parse([]byte(nextSrc))
		exists[path.Join(dir, "NEXT.md")] = true
	}
	return b, exists
}

// checkBoard checks a board against an in-memory existence set.
func checkBoard(t *testing.T, b *board.Board, exists map[string]bool, found map[string][]string) []Problem {
	t.Helper()
	problems, err := Check(context.Background(), b, func(rel string) bool { return exists[rel] }, fakeLocator(found))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return problems
}

// run builds a board from topic bytes and checks it.
func run(t *testing.T, cfg *config.Config, topics map[string]string, next string, found map[string][]string) []Problem {
	t.Helper()
	b, exists := boardFrom(t, cfg, topics, next)
	return checkBoard(t, b, exists, found)
}

// check runs a board with the default configuration and no found plans.
func check(t *testing.T, topics map[string]string, next string) []Problem {
	t.Helper()
	return run(t, defaultConfig(), topics, next, nil)
}

func requireOnly(t *testing.T, problems []Problem, rule string) Problem {
	t.Helper()
	if len(problems) != 1 {
		t.Fatalf("problems = %+v, want exactly one %q", problems, rule)
	}
	if problems[0].Rule != rule {
		t.Fatalf("rule = %q, want %q (%+v)", problems[0].Rule, rule, problems[0])
	}
	return problems[0]
}

func topicBody(kind, area, state, title string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state + "\n---\n\n# " + title + "\n"
}

func TestSlug(t *testing.T) {
	problems := check(t, map[string]string{
		"Bad_Name.md": topicBody("bug", "maps", "open", "Bad Name"),
	}, "# Next\n")
	p := requireOnly(t, problems, "slug")
	if p.Path != "docs/workbench/Bad_Name.md" || p.Line != 1 || p.Severity != Error {
		t.Errorf("problem = %+v", p)
	}
}

func TestFrontMatter(t *testing.T) {
	problems := check(t, map[string]string{
		"bad.md": "# No front matter\n",
	}, "# Next\n")
	p := requireOnly(t, problems, "front-matter")
	if p.Path != "docs/workbench/bad.md" || p.Line != 1 {
		t.Errorf("problem = %+v", p)
	}
}

func TestUnknownKey(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\ncolour: red\n---\n\n# A\n",
	}, "# Next\n")
	requireOnly(t, problems, "unknown-key")
}

func TestDuplicateKey(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\nkind: change\narea: maps\nstate: open\n---\n\n# A\n",
	}, "# Next\n")
	requireOnly(t, problems, "duplicate-key")
}

func TestMissingKey(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\n---\n\n# A\n",
	}, "# Next\n")
	requireOnly(t, problems, "missing-key")
}

func TestKind(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("nonsense", "maps", "open", "A"),
	}, "# Next\n")
	requireOnly(t, problems, "kind")
}

func TestArea(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": topicBody("bug", "nowhere", "open", "A"),
		}, "# Next\n")
		requireOnly(t, problems, "area")
	})
	t.Run("not a slug when none configured", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Areas = nil
		problems := run(t, cfg, map[string]string{
			"a.md": topicBody("bug", "Not_A_Slug", "open", "A"),
		}, "# Next\n", nil)
		requireOnly(t, problems, "area")
	})
	t.Run("any slug when none configured", func(t *testing.T) {
		cfg := defaultConfig()
		cfg.Areas = nil
		problems := run(t, cfg, map[string]string{
			"a.md": topicBody("bug", "anything-goes", "open", "A"),
		}, "# Next\n", nil)
		if len(problems) != 0 {
			t.Fatalf("problems = %+v, want none", problems)
		}
	})
}

func TestState(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("bug", "maps", "done", "A"),
	}, "# Next\n")
	requireOnly(t, problems, "state")
}

func TestPlanMissing(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("bug", "maps", "planned", "A"),
	}, "# Next\n")
	requireOnly(t, problems, "plan-missing")
}

func TestPlanUnexpected(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\nplan: docs/plans/P.md\n---\n\n# A\n",
	}, "# Next\n")
	requireOnly(t, problems, "plan-unexpected")
}

func TestPlanGone(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: planned\nplan: docs/plans/gone.md\n---\n\n# A\n",
	}, "# Next\n")
	requireOnly(t, problems, "plan-gone")
}

func TestPlanFound(t *testing.T) {
	problems := run(t, defaultConfig(), map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: planned\nplan: docs/plans/P.md\n---\n\n# A\n",
	}, "# Next\n", map[string][]string{"docs/plans/P.md": {"worktree"}})
	if len(problems) != 0 {
		t.Fatalf("problems = %+v, want none", problems)
	}
}

func TestTitle(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\njust prose\n",
		}, "# Next\n")
		requireOnly(t, problems, "title")
	})
	t.Run("more than one", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# One\n\n# Two\n",
		}, "# Next\n")
		requireOnly(t, problems, "title")
	})
}

func TestNextItem(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "# Next\n\n1. no link here\n")
	requireOnly(t, problems, "next-item")
}

func TestNextDuplicate(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "# Next\n\n1. [A](a.md)\n2. [A again](a.md)\n")
	requireOnly(t, problems, "next-duplicate")
}

func TestNextTargetsMustNameTopicFiles(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		present bool
		valid   bool
	}{
		{"plain", "a.md", false, true},
		{"dot slash", "./a.md", false, true},
		{"normalized", "sub/../a.md", false, true},
		{"no suffix", "a", false, false},
		{"subdirectory absent", "sub/a.md", false, false},
		{"subdirectory present", "sub/a.md", true, false},
		{"outside absent", "../outside/a.md", false, false},
		{"outside present", "../outside/a.md", true, false},
		{"missing topic", "b.md", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := defaultConfig()
			b, exists := boardFrom(t, cfg, map[string]string{
				"a.md": topicBody("bug", "maps", "open", "A"),
			}, "# Next\n\n1. [A]("+c.target+")\n")
			if c.present {
				exists[path.Clean(path.Join(cfg.Dir, c.target))] = true
			}
			problems := checkBoard(t, b, exists, nil)

			var nextItems, duplicates int
			for _, p := range problems {
				switch p.Rule {
				case "next-item":
					nextItems++
				case "next-duplicate":
					duplicates++
				}
			}
			wantItems := 1
			if c.valid {
				wantItems = 0
			}
			if nextItems != wantItems {
				t.Errorf("next-item count = %d, want %d (%+v)", nextItems, wantItems, problems)
			}
			if duplicates != 0 {
				t.Errorf("next-duplicate count = %d, want 0 (%+v)", duplicates, problems)
			}
		})
	}

	t.Run("invalid then valid", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": topicBody("bug", "maps", "open", "A"),
		}, "# Next\n\n1. [Bad](a)\n2. [A](a.md)\n")
		requireOnly(t, problems, "next-item")
	})
}

func TestTopicLink(t *testing.T) {
	t.Run("from a topic", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\nSee [gone](gone.md).\n",
		}, "# Next\n")
		requireOnly(t, problems, "topic-link")
	})
	t.Run("from Next free text", func(t *testing.T) {
		problems := check(t, map[string]string{
			"a.md": topicBody("bug", "maps", "open", "A"),
		}, "# Next\n\n1. [A](a.md) see [gone](gone.md)\n")
		requireOnly(t, problems, "topic-link")
	})
}

func TestCheckUsesSuppliedExistence(t *testing.T) {
	cfg := defaultConfig()
	b, exists := boardFrom(t, cfg, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\n[there](there.md) and [gone](gone.md)\n",
	}, "# Next\n")
	exists[path.Join(cfg.Dir, "there.md")] = true

	problems := checkBoard(t, b, exists, nil)
	p := requireOnly(t, problems, "topic-link")
	if !strings.Contains(p.Message, "gone.md") {
		t.Fatalf("problem = %+v, want the missing link", p)
	}

	delete(exists, path.Join(cfg.Dir, "there.md"))
	problems = checkBoard(t, b, exists, nil)
	if len(problems) != 2 {
		t.Fatalf("problems = %+v, want both links missing", problems)
	}
	for _, p := range problems {
		if p.Rule != "topic-link" {
			t.Errorf("rule = %q, want topic-link", p.Rule)
		}
	}
}

func TestTopicLinkWithRootDirectory(t *testing.T) {
	cfg := defaultConfig()
	cfg.Dir = "."
	problems := run(t, cfg, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\n[gone](missing.md) and [outside](../outside.md)\n",
	}, "# Next\n", nil)
	if len(problems) != 2 {
		t.Fatalf("problems = %+v, want topic-link and link", problems)
	}
	p := problems[0]
	if p.Rule != "topic-link" || p.Severity != Error || p.Path != "a.md" || p.Line != 9 {
		t.Errorf("topic-link problem = %+v", p)
	}
	w := problems[1]
	if w.Rule != "link" || w.Severity != Warning || w.Path != "a.md" || w.Line != 9 {
		t.Errorf("outside link problem = %+v", w)
	}
}

func TestLink(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": "---\nkind: bug\narea: maps\nstate: open\n---\n\n# A\n\nSee [plan](../plans/gone.md).\n",
	}, "# Next\n")
	p := requireOnly(t, problems, "link")
	if p.Severity != Warning {
		t.Errorf("severity = %q, want warning", p.Severity)
	}
}

func TestNextAbsent(t *testing.T) {
	problems := check(t, map[string]string{
		"a.md": topicBody("bug", "maps", "open", "A"),
	}, "")
	p := requireOnly(t, problems, "next-absent")
	if p.Severity != Warning {
		t.Errorf("severity = %q, want warning", p.Severity)
	}
}

func TestClean(t *testing.T) {
	problems := run(t, defaultConfig(), map[string]string{
		"open.md":    topicBody("bug", "maps", "open", "Open"),
		"planned.md": "---\nkind: design\narea: economy\nstate: planned\nplan: docs/plans/P.md\n---\n\n# Planned\n",
	}, "# Next\n\n1. [Open](open.md)\n\n2. [Planned](planned.md)\n",
		map[string][]string{"docs/plans/P.md": {"worktree"}})
	if len(problems) != 0 {
		t.Fatalf("problems = %+v, want none", problems)
	}
}
