package board

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Torwalt/benchpress/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Dir:   "docs/workbench",
		Kinds: []string{"bug", "change", "design", "idea"},
		Areas: []string{"economy", "maps"},
	}
}

func topicBody(kind, area, state, title string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state + "\n---\n\n# " + title + "\n"
}

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

// loadFixture builds a board with the given topics and returns it.
func loadFixture(t *testing.T, topics map[string]string, nextBody string) *Board {
	t.Helper()
	root := t.TempDir()
	for name, body := range topics {
		writeTopic(t, root, name, body)
	}
	if nextBody != "" {
		writeTopic(t, root, "NEXT.md", nextBody)
	}
	b, err := Load(root, testConfig())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return b
}

func slugs(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Slug
	}
	return out
}

func TestListingOrder(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"raised-ground.md":   topicBody("design", "maps", "planned", "Presentation on raised ground"),
		"graphics-preset.md": topicBody("idea", "economy", "open", "A graphics preset"),
		"drafts-strand.md":   topicBody("design", "economy", "open", "Drafts strand, and the player cannot help them"),
		"taxes.md":           topicBody("change", "economy", "settled", "Taxes"),
		"war-table.md":       topicBody("design", "maps", "settled", "The war table"),
	}, "# Next\n\n1. [Presentation on raised ground](raised-ground.md)\n")

	nextSlugs, entries := b.Listing(Filter{}, nil)
	if want := []string{"raised-ground"}; !reflect.DeepEqual(nextSlugs, want) {
		t.Errorf("next = %v, want %v", nextSlugs, want)
	}
	want := []string{"raised-ground", "graphics-preset", "drafts-strand", "taxes", "war-table"}
	if got := slugs(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestListingNextOrder(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"first.md":  topicBody("bug", "maps", "open", "Alpha first"),
		"second.md": topicBody("bug", "maps", "open", "Beta second"),
	}, "# Next\n\n1. [Beta second](second.md)\n2. [Alpha first](first.md)\n")

	nextSlugs, entries := b.Listing(Filter{}, nil)
	want := []string{"second", "first"}
	if !reflect.DeepEqual(nextSlugs, want) {
		t.Errorf("next = %v, want %v", nextSlugs, want)
	}
	if got := slugs(entries); !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %v, want %v", got, want)
	}
}

func TestNextTargetsMustNameTopicFiles(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		present bool
		want    string
	}{
		{"plain", "a.md", false, "a"},
		{"dot slash", "./a.md", false, "a"},
		{"normalized", "sub/../a.md", false, "a"},
		{"no suffix", "a", false, ""},
		{"subdirectory absent", "sub/a.md", false, ""},
		{"subdirectory present", "sub/a.md", true, ""},
		{"outside absent", "../outside/a.md", false, ""},
		{"outside present", "../outside/a.md", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeTopic(t, root, "a.md", topicBody("bug", "maps", "open", "A"))
			if c.present {
				full := filepath.Join(root, "docs", "workbench", filepath.FromSlash(c.target))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(full, []byte("outside\n"), 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			writeTopic(t, root, "NEXT.md", "# Next\n\n1. [A]("+c.target+")\n")
			b, err := Load(root, testConfig())
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			nextSlugs, entries := b.Listing(Filter{}, nil)
			want := []string{}
			wantIndex := -1
			if c.want != "" {
				want = []string{c.want}
				wantIndex = 0
			}
			if !reflect.DeepEqual(nextSlugs, want) {
				t.Errorf("next = %v, want %v", nextSlugs, want)
			}
			if got := slugs(entries); !reflect.DeepEqual(got, []string{"a"}) {
				t.Errorf("entries = %v, want only a", got)
			}
			if got := b.NextIndex("a"); got != wantIndex {
				t.Errorf("NextIndex(a) = %d, want %d", got, wantIndex)
			}
		})
	}

	t.Run("invalid then valid", func(t *testing.T) {
		b := loadFixture(t, map[string]string{
			"a.md": topicBody("bug", "maps", "open", "A"),
		}, "# Next\n\n1. [Bad](sub/a.md)\n2. [A](a.md)\n")
		nextSlugs, entries := b.Listing(Filter{}, nil)
		if !reflect.DeepEqual(nextSlugs, []string{"a"}) {
			t.Errorf("next = %v, want only a", nextSlugs)
		}
		if got := slugs(entries); !reflect.DeepEqual(got, []string{"a"}) {
			t.Errorf("entries = %v, want only a", got)
		}
		if got := b.NextIndex("a"); got != 1 {
			t.Errorf("NextIndex(a) = %d, want the valid item at 1", got)
		}
	})
}

func TestListingIgnoresReadmeAndSubdirectories(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"a.md":          topicBody("bug", "maps", "open", "A"),
		"README.md":     "not a topic\n",
		"notes.txt":     "not markdown\n",
		"sub/nested.md": topicBody("bug", "maps", "open", "Nested"),
	}, "# Next\n")

	if len(b.Topics) != 1 || b.Topics[0].Slug != "a" {
		t.Fatalf("Topics = %v, want only a", b.Topics)
	}
	_, entries := b.Listing(Filter{}, nil)
	if got := slugs(entries); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("entries = %v, want only a", got)
	}
}

func TestListingBrokenTopic(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"good.md": topicBody("bug", "maps", "open", "Good"),
		"bad.md":  "# No front matter\n",
	}, "# Next\n")

	_, entries := b.Listing(Filter{}, nil)
	if len(entries) != 2 {
		t.Fatalf("entries = %v, want good and bad", slugs(entries))
	}
	last := entries[len(entries)-1]
	if last.Slug != "bad" {
		t.Fatalf("last = %q, want bad", last.Slug)
	}
	if !last.Broken() || last.Issue == nil {
		t.Fatalf("bad entry not marked broken: %+v", last)
	}
	if last.Issue.Rule != "front-matter" {
		t.Errorf("issue rule = %q, want front-matter", last.Issue.Rule)
	}
}

func TestListingFilter(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"open-bug.md":       topicBody("bug", "economy", "open", "Open bug"),
		"settled-idea.md":   topicBody("idea", "maps", "settled", "Settled idea"),
		"open-change.md":    topicBody("change", "maps", "open", "Open change"),
		"planned-design.md": topicBody("design", "economy", "planned", "Planned design"),
	}, "# Next\n\n1. [Open bug](open-bug.md)\n2. [Settled idea](settled-idea.md)\n3. [Open change](open-change.md)\n")

	cases := []struct {
		name    string
		filter  Filter
		next    []string
		entries []string
	}{
		{
			name:    "state",
			filter:  Filter{States: []string{"settled"}},
			next:    []string{"settled-idea"},
			entries: []string{"settled-idea"},
		},
		{
			name:    "kinds",
			filter:  Filter{Kinds: []string{"bug", "idea"}},
			next:    []string{"open-bug", "settled-idea"},
			entries: []string{"open-bug", "settled-idea"},
		},
		{
			name:    "areas",
			filter:  Filter{Areas: []string{"maps"}},
			next:    []string{"settled-idea", "open-change"},
			entries: []string{"settled-idea", "open-change"},
		},
		{
			name:    "different filters intersect",
			filter:  Filter{States: []string{"open"}, Kinds: []string{"bug"}},
			next:    []string{"open-bug"},
			entries: []string{"open-bug"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nextSlugs, entries := b.Listing(c.filter, nil)
			if !reflect.DeepEqual(nextSlugs, c.next) {
				t.Errorf("next = %v, want %v", nextSlugs, c.next)
			}
			if got := slugs(entries); !reflect.DeepEqual(got, c.entries) {
				t.Errorf("entries = %v, want %v", got, c.entries)
			}
		})
	}
}

func TestListingPlanFound(t *testing.T) {
	planned := "---\nkind: design\narea: maps\nstate: planned\nplan: docs/plans/P.md\n---\n\n# Planned\n"
	b := loadFixture(t, map[string]string{
		"planned.md": planned,
	}, "# Next\n\n1. [Planned](planned.md)\n")

	if got := b.Plans(); !reflect.DeepEqual(got, []string{"docs/plans/P.md"}) {
		t.Fatalf("Plans = %v", got)
	}
	_, entries := b.Listing(Filter{}, map[string][]string{"docs/plans/P.md": {"worktree", "main"}})
	if len(entries) != 1 {
		t.Fatalf("entries = %v", slugs(entries))
	}
	if got := entries[0].PlanFound; !reflect.DeepEqual(got, []string{"worktree", "main"}) {
		t.Errorf("PlanFound = %v", got)
	}
}

func TestLoadMissingDir(t *testing.T) {
	b, err := Load(t.TempDir(), testConfig())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b.Topics) != 0 || len(b.Next.Items) != 0 {
		t.Errorf("board = %+v, want empty", b)
	}
}

func TestListingConfiguredAreaOrder(t *testing.T) {
	root := t.TempDir()
	writeTopic(t, root, "maps.md", topicBody("bug", "maps", "open", "Maps"))
	writeTopic(t, root, "economy.md", topicBody("bug", "economy", "open", "Economy"))
	writeTopic(t, root, "ant.md", topicBody("bug", "ant", "open", "Ant"))
	writeTopic(t, root, "zebra.md", topicBody("bug", "zebra", "open", "Zebra"))
	cfg := &config.Config{
		Dir:   "docs/workbench",
		Kinds: []string{"bug", "change", "design", "idea"},
		Areas: []string{"maps", "economy"},
	}

	b, err := Load(root, cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, entries := b.Listing(Filter{}, nil)
	if got := slugs(entries); !reflect.DeepEqual(got, []string{"maps", "economy", "ant", "zebra"}) {
		t.Errorf("order = %v, want maps, economy, ant, zebra", got)
	}
}

func TestListingGroupedStateOrder(t *testing.T) {
	b := loadFixture(t, map[string]string{
		// Titles deliberately conflict with the state order so a title-only
		// sort cannot pass this test.
		"open.md":    topicBody("design", "maps", "open", "Zulu open"),
		"settled.md": topicBody("design", "maps", "settled", "Alpha settled"),
		"planned.md": topicBody("design", "maps", "planned", "Mike planned"),
	}, "# Next\n")

	nextSlugs, entries := b.Listing(Filter{}, nil)
	if len(nextSlugs) != 0 {
		t.Fatalf("next = %v, want none", nextSlugs)
	}
	if got, want := slugs(entries), []string{"open", "settled", "planned"}; !reflect.DeepEqual(got, want) {
		t.Errorf("order = %v, want open, settled, planned", got)
	}
}

func TestListingAreasAlphabeticalWhenUnconfigured(t *testing.T) {
	root := t.TempDir()
	writeTopic(t, root, "z.md", topicBody("bug", "zebra", "open", "Z"))
	writeTopic(t, root, "a.md", topicBody("bug", "ant", "open", "A"))
	cfg := testConfig()
	cfg.Areas = nil

	b, err := Load(root, cfg)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, entries := b.Listing(Filter{}, nil)
	if got := slugs(entries); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Errorf("order = %v, want a then z", got)
	}
}

func epicTopic(kind, area, state, epic, title string) string {
	return "---\nkind: " + kind + "\narea: " + area + "\nstate: " + state + "\nepic: " + epic + "\n---\n\n# " + title + "\n"
}

func TestListingEpics(t *testing.T) {
	b := loadFixture(t, map[string]string{
		"summoner.md":   topicBody("design", "maps", "open", "The Summoner"),
		"juice.md":      epicTopic("change", "economy", "open", "summoner", "Hits have no weight"),
		"economy.md":    epicTopic("design", "economy", "settled", "summoner", "A Summoner economy"),
		"model.md":      epicTopic("change", "maps", "open", "summoner", "A Summoner model"),
		"lab.md":        epicTopic("change", "maps", "open", "summoner", "A laboratory"),
		"roads.md":      topicBody("design", "economy", "open", "Roads"),
		"orphan.md":     epicTopic("bug", "economy", "open", "gone", "An orphan"),
		"grandchild.md": epicTopic("bug", "economy", "open", "juice", "A grandchild"),
	}, "# Next\n\n1. [The Summoner](summoner.md)\n2. [A laboratory](lab.md)\n")

	nextSlugs, entries := b.Listing(Filter{}, nil)
	if want := []string{"summoner", "lab"}; !reflect.DeepEqual(nextSlugs, want) {
		t.Errorf("next = %v, want %v", nextSlugs, want)
	}
	want := []string{"summoner", "model", "juice", "economy", "lab", "grandchild", "orphan", "roads"}
	if got := slugs(entries); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	under := map[string]string{"model": "summoner", "juice": "summoner", "economy": "summoner"}
	for _, e := range entries {
		if e.Under != under[e.Slug] {
			t.Errorf("%s under %q, want %q", e.Slug, e.Under, under[e.Slug])
		}
	}
	if entries[1].Epic != "summoner" {
		t.Errorf("model epic = %q, want summoner", entries[1].Epic)
	}

	_, entries = b.Listing(Filter{Areas: []string{"economy"}}, nil)
	if want := []string{"grandchild", "orphan", "juice", "roads", "economy"}; !reflect.DeepEqual(slugs(entries), want) {
		t.Errorf("filtered order = %v, want %v", slugs(entries), want)
	}
	for _, e := range entries {
		if e.Under != "" {
			t.Errorf("%s under %q with its epic filtered out", e.Slug, e.Under)
		}
	}
}
