package next

import (
	"errors"
	"testing"
)

func TestParseItems(t *testing.T) {
	src := "# Next\n\n- [a](a.md)\n2. [b](b.md) — a note\n- [c](c.md)\n"
	n := Parse([]byte(src))
	want := []Item{
		{Line: 3, Target: "a.md", Text: "[a](a.md)"},
		{Line: 4, Target: "b.md", Text: "[b](b.md) — a note"},
		{Line: 5, Target: "c.md", Text: "[c](c.md)"},
	}
	if len(n.Items) != len(want) {
		t.Fatalf("Items = %v, want %v", n.Items, want)
	}
	for i, w := range want {
		if n.Items[i] != w {
			t.Errorf("Items[%d] = %v, want %v", i, n.Items[i], w)
		}
	}
}

func TestParseNoLink(t *testing.T) {
	n := Parse([]byte("1. plain text\n"))
	if len(n.Items) != 1 {
		t.Fatalf("Items = %v, want one", n.Items)
	}
	if n.Items[0].Target != "" {
		t.Errorf("Target = %q, want empty", n.Items[0].Target)
	}
	if n.Items[0].Text != "plain text" {
		t.Errorf("Text = %q, want %q", n.Items[0].Text, "plain text")
	}
}

func TestParseLinkForms(t *testing.T) {
	tests := []struct {
		item   string
		target string
	}{
		{"[a](a.md)", "a.md"},
		{"[a](a.md#part)", "a.md"},
		{"[a](a.md \"title\")", "a.md"},
		{"[a](<a.md>)", "a.md"},
		{"[a](https://x/y)", ""},
		{"[a](/abs.md)", ""},
		{"[a](#top)", ""},
		{"plain", ""},
	}
	for _, tt := range tests {
		n := Parse([]byte("1. " + tt.item + "\n"))
		if len(n.Items) != 1 {
			t.Fatalf("item %q: Items = %v", tt.item, n.Items)
		}
		if n.Items[0].Target != tt.target {
			t.Errorf("item %q: Target = %q, want %q", tt.item, n.Items[0].Target, tt.target)
		}
	}
}

func TestRenderNumbersBullets(t *testing.T) {
	src := "# Next\n\n- [a](a.md)\n2. [b](b.md)\n- [c](c.md)\n"
	got := Parse([]byte(src)).Render()
	want := "# Next\n\n1. [a](a.md)\n2. [b](b.md)\n3. [c](c.md)\n"
	if string(got) != want {
		t.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
}

func TestRenderGathersProse(t *testing.T) {
	src := "# Next\n\n1. [a](a.md)\n\nProse here.\n\n2. [b](b.md)\n"
	got := Parse([]byte(src)).Render()
	want := "# Next\n\n1. [a](a.md)\n2. [b](b.md)\n\nProse here.\n\n"
	if string(got) != want {
		t.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	src := "# Next\n\n1. [Presentation on raised ground](raised-ground.md) — after playtest-fixes lands\n2. [A graphics preset](graphics-preset.md)\n3. [The war table](war-table.md)\n"
	got := Parse([]byte(src)).Render()
	if string(got) != src {
		t.Errorf("Render =\n%q\nwant\n%q", got, src)
	}
}

func TestRoundTripCRLF(t *testing.T) {
	src := "# Next\r\n\r\n1. [a](a.md)\r\n2. [b](b.md)\r\n"
	got := Parse([]byte(src)).Render()
	if string(got) != src {
		t.Errorf("Render =\n%q\nwant\n%q", got, src)
	}
}

func TestRoundTripNoFinalNewline(t *testing.T) {
	src := "# Next\n\n1. [a](a.md)\n2. [b](b.md)"
	got := Parse([]byte(src)).Render()
	if string(got) != src {
		t.Errorf("Render =\n%q\nwant\n%q", got, src)
	}
}

func TestNoItems(t *testing.T) {
	src := "# Next\n\nSome prose.\n"
	n := Parse([]byte(src))
	if len(n.Items) != 0 {
		t.Fatalf("Items = %v, want none", n.Items)
	}
	if string(n.Render()) != src {
		t.Errorf("Render = %q, want unchanged", n.Render())
	}
}

func TestNew(t *testing.T) {
	if got := string(New().Render()); got != "# Next\n\n" {
		t.Errorf("empty Render = %q", got)
	}
	n := New()
	n.Add(Item{Target: "a.md", Text: "[a](a.md)"})
	if got, want := string(n.Render()), "# Next\n\n1. [a](a.md)\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestAddAppendsAfterLastLine(t *testing.T) {
	n := Parse([]byte("# Next\n\nSome prose.\n"))
	n.Add(Item{Target: "a.md", Text: "[a](a.md)"})
	if got, want := string(n.Render()), "# Next\n\nSome prose.\n1. [a](a.md)\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestAddAt(t *testing.T) {
	n := Parse([]byte("1. [a](a.md)\n2. [b](b.md)\n"))
	if err := n.AddAt(Item{Target: "x.md", Text: "[x](x.md)"}, 1); err != nil {
		t.Fatalf("AddAt: %v", err)
	}
	if got, want := string(n.Render()), "1. [x](x.md)\n2. [a](a.md)\n3. [b](b.md)\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
	if err := n.AddAt(Item{Target: "y.md", Text: "[y](y.md)"}, len(n.Items)+1); err != nil {
		t.Fatalf("AddAt end: %v", err)
	}
	if got, want := string(n.Render()), "1. [x](x.md)\n2. [a](a.md)\n3. [b](b.md)\n4. [y](y.md)\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}

func TestAddAtOutOfRange(t *testing.T) {
	n := Parse([]byte("1. [a](a.md)\n"))
	for _, at := range []int{0, -1, 3} {
		if err := n.AddAt(Item{Target: "x.md", Text: "[x](x.md)"}, at); !errors.Is(err, ErrPosition) {
			t.Errorf("AddAt(%d) error = %v, want ErrPosition", at, err)
		}
	}
	if len(n.Items) != 1 {
		t.Errorf("failed AddAt changed items: %v", n.Items)
	}
}

func TestRemove(t *testing.T) {
	n := Parse([]byte("1. [a](a.md)\n2. [b](b.md)\n3. [c](c.md)\n"))
	if err := n.Remove("b.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got, want := string(n.Render()), "1. [a](a.md)\n2. [c](c.md)\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
	if err := n.Remove("missing.md"); !errors.Is(err, ErrNoItem) {
		t.Errorf("Remove(missing) error = %v, want ErrNoItem", err)
	}
}

func TestMove(t *testing.T) {
	src := "1. [a](a.md)\n2. [b](b.md)\n3. [c](c.md)\n"
	tests := []struct {
		name   string
		target string
		at     int
		want   string
	}{
		{"first to last", "a.md", 3, "1. [b](b.md)\n2. [c](c.md)\n3. [a](a.md)\n"},
		{"last to first", "c.md", 1, "1. [c](c.md)\n2. [a](a.md)\n3. [b](b.md)\n"},
		{"middle to first", "b.md", 1, "1. [b](b.md)\n2. [a](a.md)\n3. [c](c.md)\n"},
		{"no move", "b.md", 2, src},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := Parse([]byte(src))
			if err := n.Move(tt.target, tt.at); err != nil {
				t.Fatalf("Move: %v", err)
			}
			if got := string(n.Render()); got != tt.want {
				t.Errorf("Render = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMoveErrors(t *testing.T) {
	n := Parse([]byte("1. [a](a.md)\n2. [b](b.md)\n"))
	for _, at := range []int{0, -1, 3} {
		if err := n.Move("a.md", at); !errors.Is(err, ErrPosition) {
			t.Errorf("Move(a.md, %d) error = %v, want ErrPosition", at, err)
		}
	}
	if err := n.Move("missing.md", 1); !errors.Is(err, ErrNoItem) {
		t.Errorf("Move(missing) error = %v, want ErrNoItem", err)
	}
}

func TestNoFrontHeadingPreserved(t *testing.T) {
	src := "1. [a](a.md)\n\nprose\n"
	n := Parse([]byte(src))
	n.Add(Item{Target: "b.md", Text: "[b](b.md)"})
	if got, want := string(n.Render()), "1. [a](a.md)\n2. [b](b.md)\n\nprose\n"; got != want {
		t.Errorf("Render = %q, want %q", got, want)
	}
}
