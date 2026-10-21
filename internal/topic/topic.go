// Package topic parses one topic file from bytes and edits its front matter.
// It is pure text: it reads and returns bytes, never files.
package topic

import (
	"bytes"
	"errors"
	"sort"
	"strings"
)

// ErrNoFrontMatter reports a file that does not open with a closed front matter.
var ErrNoFrontMatter = errors.New("no front matter")

// knownKeys are the front-matter keys of docs/DESIGN.md §3.
var knownKeys = map[string]bool{"kind": true, "area": true, "state": true, "plan": true}

// Field is one front-matter line.
type Field struct {
	Key   string
	Value string
	Line  int
}

// Link is one relative Markdown link found in a topic body.
type Link struct {
	Target string
	Line   int
}

// Issue is one structural problem, named by a lint rule of docs/DESIGN.md §6.
type Issue struct {
	Rule    string
	Line    int
	Message string
}

// Topic is a parsed topic file.
type Topic struct {
	Fields    []Field
	Title     string
	TitleLine int
	Links     []Link
	Body      []byte // everything after the closing front-matter line
}

// Field returns the front-matter field for key and whether it is present.
func (t Topic) Field(key string) (Field, bool) {
	for _, f := range t.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Parse reads one topic file. It reports the structural problems of §6 that can
// be seen without the configuration: front-matter, unknown-key, duplicate-key,
// missing-key and title. Value checks are lint's job.
func Parse(src []byte) (Topic, []Issue) {
	var t Topic
	lines := splitLines(src)

	if len(lines) == 0 || !bytes.Equal(content(lines[0]), []byte("---")) {
		return t, []Issue{{Rule: "front-matter", Line: 1, Message: "file does not start with ---"}}
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(content(lines[i]), []byte("---")) {
			closing = i
			break
		}
	}
	if closing < 0 {
		return t, []Issue{{Rule: "front-matter", Line: 1, Message: "front matter is not closed"}}
	}
	t.Body = bytes.Join(lines[closing+1:], nil)

	var issues []Issue
	seen := make(map[string]bool)
	for i := 1; i < closing; i++ {
		line := i + 1
		key, value, ok := splitField(string(content(lines[i])))
		if !ok {
			issues = append(issues, Issue{Rule: "front-matter", Line: line, Message: "line is not key: value"})
			continue
		}
		if !knownKeys[key] {
			issues = append(issues, Issue{Rule: "unknown-key", Line: line, Message: `unknown key "` + key + `"`})
			continue
		}
		if seen[key] {
			issues = append(issues, Issue{Rule: "duplicate-key", Line: line, Message: `duplicate key "` + key + `"`})
			continue
		}
		seen[key] = true
		t.Fields = append(t.Fields, Field{Key: key, Value: value, Line: line})
	}
	for _, key := range []string{"kind", "area", "state"} {
		if !seen[key] {
			issues = append(issues, Issue{Rule: "missing-key", Line: 1, Message: `missing key "` + key + `"`})
		}
	}

	firstLine := 0
	for i := closing + 1; i < len(lines); i++ {
		if strings.TrimSpace(string(content(lines[i]))) != "" {
			firstLine = i + 1
			break
		}
	}

	type heading struct {
		line int
		text string
	}
	var headings []heading
	inFence := false
	var fenceChar byte
	fenceLen := 0
	for i := closing + 1; i < len(lines); i++ {
		line := i + 1
		text := string(content(lines[i]))
		if inFence {
			if isFenceClose(text, fenceChar, fenceLen) {
				inFence = false
			}
			continue
		}
		if ch, n, ok := fenceOpen(text); ok {
			inFence, fenceChar, fenceLen = true, ch, n
			continue
		}
		if title, ok := heading1(text); ok {
			headings = append(headings, heading{line: line, text: title})
		}
		collectLinks(text, line, &t.Links)
	}

	if len(headings) == 0 {
		line := 1
		if firstLine != 0 {
			line = firstLine
		}
		issues = append(issues, Issue{Rule: "title", Line: line, Message: "missing level-one heading"})
	} else {
		t.Title = headings[0].text
		t.TitleLine = headings[0].line
		if strings.TrimSpace(t.Title) == "" {
			issues = append(issues, Issue{Rule: "title", Line: headings[0].line, Message: "level-one heading has no title"})
		}
		if firstLine != headings[0].line {
			issues = append(issues, Issue{Rule: "title", Line: headings[0].line, Message: "level-one heading is not the first content"})
		}
		for _, extra := range headings[1:] {
			issues = append(issues, Issue{Rule: "title", Line: extra.line, Message: "more than one level-one heading"})
		}
	}

	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Line < issues[j].Line })
	return t, issues
}

// Set replaces key's front-matter line with key: value, appending it just before
// the closing --- when absent. Only that line's bytes change.
func Set(src []byte, key, value string) ([]byte, error) {
	lines := splitLines(src)
	closing, err := frontMatter(lines)
	if err != nil {
		return nil, err
	}
	edited := []byte(key + ": " + value)
	for i := 1; i < closing; i++ {
		k, _, ok := splitField(string(content(lines[i])))
		if ok && k == key {
			out := make([][]byte, len(lines))
			copy(out, lines)
			out[i] = append(edited, terminator(lines[i])...)
			return bytes.Join(out, nil), nil
		}
	}
	term := terminator(lines[closing])
	if len(term) == 0 {
		// An unterminated closing --- has no style of its own; keep the file's.
		term = terminator(lines[closing-1])
	}
	if len(term) == 0 {
		term = []byte("\n")
	}
	out := make([][]byte, 0, len(lines)+1)
	out = append(out, lines[:closing]...)
	out = append(out, append(edited, term...))
	out = append(out, lines[closing:]...)
	return bytes.Join(out, nil), nil
}

// Remove drops key's front-matter line, leaving every other byte. It is a no-op
// when the key is absent.
func Remove(src []byte, key string) ([]byte, error) {
	lines := splitLines(src)
	closing, err := frontMatter(lines)
	if err != nil {
		return nil, err
	}
	for i := 1; i < closing; i++ {
		k, _, ok := splitField(string(content(lines[i])))
		if ok && k == key {
			out := make([][]byte, 0, len(lines)-1)
			out = append(out, lines[:i]...)
			out = append(out, lines[i+1:]...)
			return bytes.Join(out, nil), nil
		}
	}
	return append([]byte(nil), src...), nil
}

// ValidSlug reports whether name is a valid topic file name: §3's
// ^[a-z0-9]+(-[a-z0-9]+)*\.md$.
func ValidSlug(name string) bool {
	base, ok := strings.CutSuffix(name, ".md")
	if !ok || base == "" {
		return false
	}
	for _, part := range strings.Split(base, "-") {
		if part == "" {
			return false
		}
		for i := 0; i < len(part); i++ {
			if c := part[i]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}

// Slugify turns a title into a slug: lower case, every run outside a-z0-9 one
// hyphen, leading and trailing hyphens dropped.
func Slugify(title string) string {
	var b strings.Builder
	hyphen := true // true drops leading hyphens
	for _, r := range strings.ToLower(title) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			hyphen = false
		} else if !hyphen {
			b.WriteByte('-')
			hyphen = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// splitLines cuts src after each newline; the last line may lack one. Joining
// the result with nothing returns src's bytes.
func splitLines(src []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lines = append(lines, src[start:i+1])
			start = i + 1
		}
	}
	if start < len(src) {
		lines = append(lines, src[start:])
	}
	return lines
}

// content is a line without its terminator.
func content(line []byte) []byte {
	s := line
	if n := len(s); n > 0 && s[n-1] == '\n' {
		s = s[:n-1]
	}
	if n := len(s); n > 0 && s[n-1] == '\r' {
		s = s[:n-1]
	}
	return s
}

// terminator is a line's line ending, empty when it has none.
func terminator(line []byte) []byte {
	switch {
	case bytes.HasSuffix(line, []byte("\r\n")):
		return []byte("\r\n")
	case bytes.HasSuffix(line, []byte("\n")):
		return []byte("\n")
	}
	return nil
}

// frontMatter returns the index of the closing --- line, or ErrNoFrontMatter.
func frontMatter(lines [][]byte) (int, error) {
	if len(lines) == 0 || !bytes.Equal(content(lines[0]), []byte("---")) {
		return 0, ErrNoFrontMatter
	}
	for i := 1; i < len(lines); i++ {
		if bytes.Equal(content(lines[i]), []byte("---")) {
			return i, nil
		}
	}
	return 0, ErrNoFrontMatter
}

// splitField reads one key: value line: the key is everything before the first
// colon-and-space, the value the trimmed remainder.
func splitField(line string) (key, value string, ok bool) {
	i := strings.Index(line, ": ")
	if i <= 0 {
		return "", "", false
	}
	return line[:i], strings.TrimSpace(line[i+2:]), true
}

// heading1 returns a line's level-one heading text, if it is one.
func heading1(line string) (string, bool) {
	i := 0
	for i < len(line) && line[i] == ' ' && i < 3 {
		i++
	}
	s := strings.TrimRight(line[i:], " \t")
	if s == "#" {
		return "", true
	}
	if strings.HasPrefix(s, "# ") {
		return strings.TrimSpace(s[2:]), true
	}
	return "", false
}

// fenceOpen reports a line that opens a ``` or ~~~ fence.
func fenceOpen(line string) (char byte, length int, ok bool) {
	char, length, _, ok = fenceMarker(line)
	return char, length, ok
}

// isFenceClose reports a line that closes a fence of char and at least length.
func isFenceClose(line string, char byte, length int) bool {
	c, n, rest, ok := fenceMarker(line)
	return ok && c == char && n >= length && strings.TrimSpace(rest) == ""
}

// fenceMarker reads a fence line: up to three leading spaces, three or more
// backticks or tildes, and the rest of the line.
func fenceMarker(line string) (char byte, length int, rest string, ok bool) {
	i := 0
	for i < len(line) && line[i] == ' ' && i < 3 {
		i++
	}
	if i >= len(line) || line[i] != '`' && line[i] != '~' {
		return 0, 0, "", false
	}
	c := line[i]
	j := i
	for j < len(line) && line[j] == c {
		j++
	}
	if j-i < 3 {
		return 0, 0, "", false
	}
	return c, j - i, line[j:], true
}

// LineLinks returns the checkable relative links on one line of Markdown,
// ignoring code spans, each numbered with line.
func LineLinks(line string, number int) []Link {
	var links []Link
	collectLinks(line, number, &links)
	return links
}

// collectLinks appends every relative inline link on line, ignoring code.
func collectLinks(line string, number int, out *[]Link) {
	for i := 0; i < len(line); {
		switch line[i] {
		case '`':
			i = skipCodeSpan(line, i)
		case '[':
			target, end, ok := parseLink(line, i)
			if !ok {
				i++
				continue
			}
			if t, keep := cleanTarget(target); keep {
				*out = append(*out, Link{Target: t, Line: number})
			}
			i = end
		default:
			i++
		}
	}
}

// skipCodeSpan returns the index just past the code span opened at i, or past
// the opening run when it has no match.
func skipCodeSpan(line string, i int) int {
	j := i
	for j < len(line) && line[j] == '`' {
		j++
	}
	n := j - i
	for k := j; k < len(line); {
		if line[k] != '`' {
			k++
			continue
		}
		m := k
		for m < len(line) && line[m] == '`' {
			m++
		}
		if m-k == n {
			return m
		}
		k = m
	}
	return j
}

// parseLink reads an inline link [text](target "title") at i, returning the
// raw target and the index just past the closing parenthesis.
func parseLink(line string, i int) (target string, end int, ok bool) {
	closeText := strings.IndexByte(line[i+1:], ']')
	if closeText < 0 {
		return "", 0, false
	}
	j := i + 1 + closeText
	if j+1 >= len(line) || line[j+1] != '(' {
		return "", 0, false
	}
	closeTarget := strings.IndexByte(line[j+2:], ')')
	if closeTarget < 0 {
		return "", 0, false
	}
	m := j + 2 + closeTarget
	raw, ok := linkTarget(line[j+2 : m])
	if !ok {
		return "", 0, false
	}
	return raw, m + 1, true
}

// linkTarget reads the target out of a link's parentheses, dropping a title.
func linkTarget(inside string) (string, bool) {
	s := strings.TrimSpace(inside)
	if s == "" {
		return "", false
	}
	if s[0] == '<' {
		close := strings.IndexByte(s, '>')
		if close < 0 {
			return "", false
		}
		target := s[1:close]
		if strings.TrimSpace(target) == "" {
			return "", false
		}
		return target, true
	}
	target, _, _ := strings.Cut(s, " ")
	return target, target != ""
}

// cleanTarget reports the target of a checkable relative link: no fragment, and
// false for schemes, absolute paths and bare fragments.
func cleanTarget(raw string) (string, bool) {
	t := strings.TrimSpace(raw)
	if t == "" || t[0] == '#' || t[0] == '/' || hasScheme(t) {
		return "", false
	}
	if i := strings.IndexByte(t, '#'); i >= 0 {
		t = t[:i]
	}
	if t == "" {
		return "", false
	}
	return t, true
}

// hasScheme reports a URI scheme before any slash.
func hasScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '/':
			return false
		case c == ':':
			if i == 0 || !isLetter(s[0]) {
				return false
			}
			for j := 1; j < i; j++ {
				if d := s[j]; !isLetter(d) && !isDigit(d) && d != '+' && d != '-' && d != '.' {
					return false
				}
			}
			return true
		}
	}
	return false
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
