// File- and Next-writing operations of docs/DESIGN.md §7. They edit topics
// surgically and write atomically; the files are the only thing changed.
package board

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Torwalt/benchpress/internal/next"
	"github.com/Torwalt/benchpress/internal/topic"
)

// New creates a topic file from title with the given kind and area and state
// open, and returns its repository-relative path. It fails when the file
// exists, when kind or area is not allowed, or when the title makes no slug.
// When toNext, the new topic is appended to Next.
func (b *Board) New(title, kind, area string, toNext bool) (string, error) {
	if !inList(b.Config.Kinds, kind) {
		return "", fmt.Errorf("kind %q is not allowed", kind)
	}
	if !b.areaAllowed(area) {
		return "", fmt.Errorf("area %q is not allowed", area)
	}
	slug := topic.Slugify(title)
	if slug == "" {
		return "", fmt.Errorf("title %q makes no slug", title)
	}
	rel := path.Join(b.Config.Dir, slug+".md")
	full := filepath.Join(b.Root, filepath.FromSlash(rel))
	if _, err := os.Stat(full); err == nil {
		return "", fmt.Errorf("%s already exists", rel)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := writeAtomic(full, topicFile(title, kind, area), 0o644); err != nil {
		return "", err
	}
	if toNext {
		if err := b.addNextItem(slug, title, 0); err != nil {
			return "", err
		}
	}
	return rel, nil
}

// SetState sets a topic's state. State planned requires plan; any other state
// removes plan. Only the front-matter lines that change are rewritten.
func (b *Board) SetState(slug, state, plan string) error {
	t, err := b.requireTopic(slug)
	if err != nil {
		return err
	}
	if !validState(state) {
		return fmt.Errorf("state %q is not open, settled or planned", state)
	}
	if state == "planned" && plan == "" {
		return errors.New("state planned requires a plan")
	}
	full := filepath.Join(b.Root, filepath.FromSlash(t.Path))
	src, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	if src, err = topic.Set(src, "state", state); err != nil {
		return err
	}
	if state == "planned" {
		src, err = topic.Set(src, "plan", plan)
	} else {
		src, err = topic.Remove(src, "plan")
	}
	if err != nil {
		return err
	}
	return writeAtomic(full, src, 0o644)
}

// NextAdd adds a topic to Next at the 1-based position at, or at the end when
// at is not positive. It fails when the topic is unknown or already in Next.
func (b *Board) NextAdd(slug string, at int) error {
	t, err := b.requireTopic(slug)
	if err != nil {
		return err
	}
	if b.nextIndex(slug) >= 0 {
		return fmt.Errorf("topic %q is already in Next", slug)
	}
	return b.addNextItem(t.Slug, t.Title(), at)
}

// NextRemove removes a topic from Next. It fails when the topic is unknown or
// is not in Next.
func (b *Board) NextRemove(slug string) error {
	if _, err := b.requireTopic(slug); err != nil {
		return err
	}
	i := b.nextIndex(slug)
	if i < 0 {
		return fmt.Errorf("topic %q is not in Next", slug)
	}
	if err := b.Next.Remove(b.Next.Items[i].Target); err != nil {
		return err
	}
	return b.writeNext()
}

// NextMove moves a topic to the 1-based position at. It fails when the topic
// is unknown, is not in Next, or at is out of range.
func (b *Board) NextMove(slug string, at int) error {
	if _, err := b.requireTopic(slug); err != nil {
		return err
	}
	i := b.nextIndex(slug)
	if i < 0 {
		return fmt.Errorf("topic %q is not in Next", slug)
	}
	if err := b.Next.Move(b.Next.Items[i].Target, at); err != nil {
		return err
	}
	return b.writeNext()
}

// Remove deletes a topic file and takes it out of Next, returning the paths of
// the topics that still link to it. It fails on an unknown slug.
func (b *Board) Remove(slug string) ([]string, error) {
	t, err := b.requireTopic(slug)
	if err != nil {
		return nil, err
	}
	linked := b.linkedBy(t)
	full := filepath.Join(b.Root, filepath.FromSlash(t.Path))
	if err := os.Remove(full); err != nil {
		return nil, err
	}
	if i := b.nextIndex(slug); i >= 0 {
		if err := b.Next.Remove(b.Next.Items[i].Target); err != nil {
			return nil, err
		}
		if err := b.writeNext(); err != nil {
			return nil, err
		}
	}
	return linked, nil
}

// requireTopic returns the loaded topic for slug, accepting a trailing .md.
func (b *Board) requireTopic(slug string) (*Topic, error) {
	slug = strings.TrimSuffix(slug, ".md")
	for _, t := range b.Topics {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown topic %q", slug)
}

// nextIndex returns the position of slug in Next, or -1.
func (b *Board) nextIndex(slug string) int {
	slug = strings.TrimSuffix(slug, ".md")
	for i, item := range b.Next.Items {
		if targetSlug(item.Target) == slug {
			return i
		}
	}
	return -1
}

// addNextItem appends or inserts a topic's link in Next and rewrites the file.
func (b *Board) addNextItem(slug, title string, at int) error {
	item := next.Item{Target: slug + ".md", Text: nextText(slug, title)}
	if at > 0 {
		if err := b.Next.AddAt(item, at); err != nil {
			return err
		}
	} else {
		b.Next.Add(item)
	}
	return b.writeNext()
}

// writeNext renders and writes NEXT.md atomically.
func (b *Board) writeNext() error {
	full := filepath.Join(b.Root, filepath.FromSlash(b.Config.Dir), "NEXT.md")
	return writeAtomic(full, b.Next.Render(), 0o644)
}

// linkedBy returns the paths of the other topics linking to target.
func (b *Board) linkedBy(target *Topic) []string {
	dir := path.Clean(b.Config.Dir)
	var out []string
	for _, t := range b.Topics {
		if t.Slug == target.Slug {
			continue
		}
		for _, link := range t.Parsed.Links {
			if path.Clean(path.Join(dir, link.Target)) == target.Path {
				out = append(out, t.Path)
				break
			}
		}
	}
	return out
}

// areaAllowed reports whether area is in the configured list, or is a slug when
// the configuration names no areas.
func (b *Board) areaAllowed(area string) bool {
	if area == "" {
		return false
	}
	if len(b.Config.Areas) > 0 {
		return inList(b.Config.Areas, area)
	}
	return topic.ValidSlug(area + ".md")
}

// topicFile is a new topic's bytes: front matter, title, empty Now and Open.
func topicFile(title, kind, area string) []byte {
	return []byte("---\nkind: " + kind + "\narea: " + area +
		"\nstate: open\n---\n\n# " + title + "\n\n**Now:**\n\n**Open:**\n")
}

// nextText is a topic's Next item content: its link, with the title as text.
func nextText(slug, title string) string {
	if title == "" {
		title = slug
	}
	return "[" + title + "](" + slug + ".md)"
}

func validState(s string) bool {
	switch s {
	case "open", "settled", "planned":
		return true
	}
	return false
}

// writeAtomic writes data through a temporary file in the same directory and a
// rename, keeping the destination's mode when it exists.
func writeAtomic(name string, data []byte, mode fs.FileMode) error {
	if info, err := os.Stat(name); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(name)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, name)
}
