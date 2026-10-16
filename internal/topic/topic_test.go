package topic

import (
	"bytes"
	"errors"
	"testing"
)

const valid = "---\nkind: design\narea: army\nstate: planned\nplan: docs/plans/PLAYTEST_FIXES.md\n---\n\n# Drafts strand, and the player cannot help them\n\n*Design, from the 2026-10-13 playtest.* What was said, dated.\n\n**Now:** what the build does today, with landmarks.\n\n**Open:**\n\n- the questions still to settle\n"

func TestParseValid(t *testing.T) {
	got, issues := Parse([]byte(valid))
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	wantFields := []Field{
		{Key: "kind", Value: "design", Line: 2},
		{Key: "area", Value: "army", Line: 3},
		{Key: "state", Value: "planned", Line: 4},
		{Key: "plan", Value: "docs/plans/PLAYTEST_FIXES.md", Line: 5},
	}
	if len(got.Fields) != len(wantFields) {
		t.Fatalf("Fields = %v, want %v", got.Fields, wantFields)
	}
	for i, f := range wantFields {
		if got.Fields[i] != f {
			t.Errorf("Fields[%d] = %v, want %v", i, got.Fields[i], f)
		}
	}
	if got.Title != "Drafts strand, and the player cannot help them" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.TitleLine != 8 {
		t.Errorf("TitleLine = %d, want 8", got.TitleLine)
	}
	if len(got.Links) != 0 {
		t.Errorf("Links = %v, want none", got.Links)
	}
}

func TestParseBody(t *testing.T) {
	got, _ := Parse([]byte(valid))
	want := "\n# Drafts strand, and the player cannot help them\n\n*Design, from the 2026-10-13 playtest.* What was said, dated.\n\n**Now:** what the build does today, with landmarks.\n\n**Open:**\n\n- the questions still to settle\n"
	if string(got.Body) != want {
		t.Errorf("Body = %q, want %q", got.Body, want)
	}
}

func TestParseBodyWithoutFrontMatter(t *testing.T) {
	got, _ := Parse([]byte("# just a heading\n"))
	if got.Body != nil {
		t.Errorf("Body = %q, want nil", got.Body)
	}
}

func TestParseField(t *testing.T) {
	got, _ := Parse([]byte(valid))
	f, ok := got.Field("area")
	if !ok || f.Value != "army" || f.Line != 3 {
		t.Errorf("Field(area) = %v, %v", f, ok)
	}
	if _, ok := got.Field("plan"); !ok {
		t.Error("Field(plan) not found")
	}
	if _, ok := got.Field("missing"); ok {
		t.Error("Field(missing) found")
	}
}

func TestParseStructuralIssues(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Issue
	}{
		{
			name: "no front matter",
			src:  "# Title\n",
			want: []Issue{{Rule: "front-matter", Line: 1}},
		},
		{
			name: "unclosed front matter",
			src:  "---\nkind: bug\narea: maps\nstate: open\n\n# Title\n",
			want: []Issue{{Rule: "front-matter", Line: 1}},
		},
		{
			name: "bad field line",
			src:  "---\nkind: bug\nnot a field\narea: maps\nstate: open\n---\n\n# Title\n",
			want: []Issue{{Rule: "front-matter", Line: 3}},
		},
		{
			name: "unknown key",
			src:  "---\nkind: bug\nfoo: bar\narea: maps\nstate: open\n---\n\n# Title\n",
			want: []Issue{{Rule: "unknown-key", Line: 3}},
		},
		{
			name: "duplicate key",
			src:  "---\nkind: bug\nkind: idea\narea: maps\nstate: open\n---\n\n# Title\n",
			want: []Issue{{Rule: "duplicate-key", Line: 3}},
		},
		{
			name: "missing state",
			src:  "---\nkind: bug\narea: maps\n---\n\n# Title\n",
			want: []Issue{{Rule: "missing-key", Line: 1}},
		},
		{
			name: "missing all three",
			src:  "---\n---\n\n# Title\n",
			want: []Issue{
				{Rule: "missing-key", Line: 1},
				{Rule: "missing-key", Line: 1},
				{Rule: "missing-key", Line: 1},
			},
		},
		{
			name: "no title",
			src:  "---\nkind: bug\narea: maps\nstate: open\n---\n\nSome prose.\n",
			want: []Issue{{Rule: "title", Line: 7}},
		},
		{
			name: "two titles",
			src:  "---\nkind: bug\narea: maps\nstate: open\n---\n\n# One\n\n# Two\n",
			want: []Issue{{Rule: "title", Line: 9}},
		},
		{
			name: "heading not first",
			src:  "---\nkind: bug\narea: maps\nstate: open\n---\n\nIntro\n# One\n",
			want: []Issue{{Rule: "title", Line: 8}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, issues := Parse([]byte(tt.src))
			if len(issues) != len(tt.want) {
				t.Fatalf("issues = %v, want %v", issues, tt.want)
			}
			for i, w := range tt.want {
				got := issues[i]
				if got.Rule != w.Rule || got.Line != w.Line {
					t.Errorf("issue[%d] = {%s %d}, want {%s %d}", i, got.Rule, got.Line, w.Rule, w.Line)
				}
				if got.Message == "" {
					t.Errorf("issue[%d] has empty message", i)
				}
			}
		})
	}
}

func TestParseLinks(t *testing.T) {
	src := "---\nkind: bug\narea: maps\nstate: open\n---\n\n" +
		"# Links\n\n" +
		"A [real](real.md) link.\n\n" +
		"`[inline](inline.md)` and ``[double](double.md)``.\n\n" +
		"```\n[fenced](fenced.md)\n```\n\n" +
		"~~~text\n[tildes](tildes.md)\n~~~\n"
	got, issues := Parse([]byte(src))
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	want := []Link{{Target: "real.md", Line: 9}}
	if len(got.Links) != len(want) {
		t.Fatalf("Links = %v, want %v", got.Links, want)
	}
	for i, l := range want {
		if got.Links[i] != l {
			t.Errorf("Links[%d] = %v, want %v", i, got.Links[i], l)
		}
	}
}

func TestParseLinkTargets(t *testing.T) {
	src := "---\nkind: bug\narea: maps\nstate: open\n---\n\n" +
		"# Links\n\n" +
		"[title](a.md \"A title\") [frag](b.md#part) [bare](#top) [web](https://x/y) [abs](/etc/x) [mail](mailto:a@b)\n"
	got, issues := Parse([]byte(src))
	if len(issues) != 0 {
		t.Fatalf("issues = %v, want none", issues)
	}
	want := []Link{{Target: "a.md", Line: 9}, {Target: "b.md", Line: 9}}
	if len(got.Links) != len(want) {
		t.Fatalf("Links = %v, want %v", got.Links, want)
	}
	for i, l := range want {
		if got.Links[i] != l {
			t.Errorf("Links[%d] = %v, want %v", i, got.Links[i], l)
		}
	}
}

func TestSetReplaceAndAppend(t *testing.T) {
	base := "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Title\n"
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name:  "replace",
			key:   "state",
			value: "settled",
			want:  "---\nkind: bug\narea: maps\nstate: settled\n---\n\n# Title\n",
		},
		{
			name:  "append",
			key:   "plan",
			value: "docs/plans/x.md",
			want:  "---\nkind: bug\narea: maps\nstate: open\nplan: docs/plans/x.md\n---\n\n# Title\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Set([]byte(base), tt.key, tt.value)
			if err != nil {
				t.Fatalf("Set: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Set =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestSetCRLF(t *testing.T) {
	base := "---\r\nkind: bug\r\narea: maps\r\nstate: open\r\n---\r\n\r\n# Title\r\n"
	got, err := Set([]byte(base), "state", "settled")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\r\nkind: bug\r\narea: maps\r\nstate: settled\r\n---\r\n\r\n# Title\r\n"
	if string(got) != want {
		t.Errorf("Set =\n%q\nwant\n%q", got, want)
	}
	if !bytes.Contains(got, []byte("\r\n")) {
		t.Error("CRLF was not preserved")
	}
}

func TestSetAppendCRLF(t *testing.T) {
	base := "---\r\nkind: bug\r\narea: maps\r\nstate: planned\r\n---\r\n\r\n# Title\r\n"
	got, err := Set([]byte(base), "plan", "docs/plans/x.md")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\r\nkind: bug\r\narea: maps\r\nstate: planned\r\nplan: docs/plans/x.md\r\n---\r\n\r\n# Title\r\n"
	if string(got) != want {
		t.Errorf("Set =\n%q\nwant\n%q", got, want)
	}
}

func TestSetPreservesOtherBytes(t *testing.T) {
	base := "---\r\nkind: bug  \r\narea: maps\r\nstate: open\r\n---\r\n\r\n# T\r\n\r\nNo final newline"
	got, err := Set([]byte(base), "state", "settled")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	want := "---\r\nkind: bug  \r\narea: maps\r\nstate: settled\r\n---\r\n\r\n# T\r\n\r\nNo final newline"
	if string(got) != want {
		t.Errorf("Set =\n%q\nwant\n%q", got, want)
	}
}

func TestRemove(t *testing.T) {
	base := "---\nkind: bug\narea: maps\nstate: planned\nplan: docs/plans/x.md\n---\n\n# Title\n"
	got, err := Remove([]byte(base), "plan")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	want := "---\nkind: bug\narea: maps\nstate: planned\n---\n\n# Title\n"
	if string(got) != want {
		t.Errorf("Remove =\n%q\nwant\n%q", got, want)
	}
}

func TestRemoveAbsent(t *testing.T) {
	base := "---\nkind: bug\narea: maps\nstate: open\n---\n\n# Title\n"
	got, err := Remove([]byte(base), "plan")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if string(got) != base {
		t.Errorf("Remove = %q, want unchanged", got)
	}
}

func TestEditNoFrontMatter(t *testing.T) {
	src := []byte("# Title\n")
	if _, err := Set(src, "kind", "bug"); !errors.Is(err, ErrNoFrontMatter) {
		t.Errorf("Set error = %v, want ErrNoFrontMatter", err)
	}
	if _, err := Remove(src, "kind"); !errors.Is(err, ErrNoFrontMatter) {
		t.Errorf("Remove error = %v, want ErrNoFrontMatter", err)
	}
}

func TestValidSlug(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"drafts-strand.md", true},
		{"draft2.md", true},
		{"2026-10-16.md", true},
		{"drafts-strand", false},
		{"Drafts.md", false},
		{"drafts_strand.md", false},
		{"drafts--strand.md", false},
		{"-drafts.md", false},
		{"drafts-.md", false},
		{".md", false},
		{"docs/drafts.md", false},
		{"drafts.txt", false},
	}
	for _, tt := range tests {
		if got := ValidSlug(tt.name); got != tt.want {
			t.Errorf("ValidSlug(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Drafts strand, and the player cannot help them", "drafts-strand-and-the-player-cannot-help-them"},
		{"Hello, World!", "hello-world"},
		{"Fix 2 bugs", "fix-2-bugs"},
		{"  --Leading and trailing--  ", "leading-and-trailing"},
		{"Été à Paris", "t-paris"},
		{"Über", "ber"},
		{"2026-10-16", "2026-10-16"},
		{"***", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := Slugify(tt.title); got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}
