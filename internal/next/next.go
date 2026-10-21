// Package next parses NEXT.md into its items and rewrites the list block.
// It is pure text: it reads and returns bytes, never files.
package next

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrPosition reports a position outside the list.
var ErrPosition = errors.New("position out of range")

// ErrNoItem reports a target that is not in Next.
var ErrNoItem = errors.New("item not in Next")

// Item is one list line of NEXT.md.
type Item struct {
	Line   int    // 1-based line in the parsed source, 0 when added
	Target string // the leading link's relative target, empty when there is none
	Text   string // the item's content without its bullet or number

	term string // the item's own line ending, empty when the source had none
}

// Next is a parsed NEXT.md: its items and the lines that are not items.
type Next struct {
	Items []Item

	head     [][]byte // non-item lines before the first item
	tail     [][]byte // non-item lines after the first item
	nl       []byte   // the file's line ending
	trailing bool     // whether the source ended with a line ending
}

// New returns an empty Next for a missing file. It renders as "# Next" and a
// blank line, followed by the block.
func New() Next {
	return Next{
		head:     [][]byte{[]byte("# Next\n"), []byte("\n")},
		nl:       []byte("\n"),
		trailing: true,
	}
}

// Parse reads NEXT.md: every list line as an item, every other line kept as it
// was.
func Parse(src []byte) Next {
	n := Next{nl: newline(src), trailing: len(src) > 0 && src[len(src)-1] == '\n'}
	first := -1
	for i, line := range splitLines(src) {
		body := content(line)
		text, target, ok := parseItem(string(body))
		if !ok {
			if first < 0 {
				n.head = append(n.head, line)
			} else {
				n.tail = append(n.tail, line)
			}
			continue
		}
		if first < 0 {
			first = i
		}
		n.Items = append(n.Items, Item{
			Line:   i + 1,
			Target: target,
			Text:   text,
			term:   string(line[len(body):]),
		})
	}
	return n
}

// Add puts item at the end of the list.
func (n *Next) Add(item Item) {
	n.Items = append(n.Items, item)
}

// AddAt inserts item at the 1-based position at; len(Items)+1 is the end.
func (n *Next) AddAt(item Item, at int) error {
	if at < 1 || at > len(n.Items)+1 {
		return fmt.Errorf("%w: %d", ErrPosition, at)
	}
	n.Items = append(n.Items, Item{})
	copy(n.Items[at:], n.Items[at-1:])
	n.Items[at-1] = item
	return nil
}

// Remove drops the item whose Target is target.
func (n *Next) Remove(target string) error {
	for i, item := range n.Items {
		if item.Target == target {
			n.Items = append(n.Items[:i], n.Items[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrNoItem, target)
}

// Move moves the item whose Target is target to the 1-based position at.
func (n *Next) Move(target string, at int) error {
	if at < 1 || at > len(n.Items) {
		return fmt.Errorf("%w: %d", ErrPosition, at)
	}
	from := -1
	for i, item := range n.Items {
		if item.Target == target {
			from = i
			break
		}
	}
	if from < 0 {
		return fmt.Errorf("%w: %s", ErrNoItem, target)
	}
	item := n.Items[from]
	n.Items = append(n.Items[:from], n.Items[from+1:]...)
	n.Items = append(n.Items, Item{})
	copy(n.Items[at:], n.Items[at-1:])
	n.Items[at-1] = item
	return nil
}

// Render writes the items as one contiguous numbered block where the first item
// stood, every non-item line kept.
func (n Next) Render() []byte {
	out := make([][]byte, 0, len(n.head)+len(n.Items)+len(n.tail)+1)
	out = append(out, n.head...)
	if len(n.Items) > 0 && len(n.head) > 0 && !bytes.HasSuffix(n.head[len(n.head)-1], []byte("\n")) {
		out = append(out, n.nl)
	}
	for i, item := range n.Items {
		line := []byte(strconv.Itoa(i+1) + ". " + item.Text)
		final := i == len(n.Items)-1 && len(n.tail) == 0
		term := item.term
		if final && !n.trailing {
			// The source did not end with a newline, so its final line must not.
			term = ""
		} else if term == "" {
			term = string(n.nl)
		}
		line = append(line, term...)
		out = append(out, line)
	}
	out = append(out, n.tail...)
	return bytes.Join(out, nil)
}

// parseItem reads a list line: "- " or "N. " at the start, then the content and
// the target of a leading link if there is one.
func parseItem(line string) (text, target string, ok bool) {
	if strings.HasPrefix(line, "- ") {
		text = line[2:]
	} else if n := orderedMarker(line); n > 0 {
		text = line[n:]
	} else {
		return "", "", false
	}
	return text, leadingTarget(text), true
}

// orderedMarker returns the length of an "N. " marker at the start of line, or
// 0 when there is none.
func orderedMarker(line string) int {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i > 0 && strings.HasPrefix(line[i:], ". ") {
		return i + 2
	}
	return 0
}

// leadingTarget reads the relative target of the inline link at the very start
// of text, dropping a title and a fragment, and returns "" for anything that is
// not a relative path.
func leadingTarget(text string) string {
	if !strings.HasPrefix(text, "[") {
		return ""
	}
	j := 1
	for j < len(text) && (text[j] != ']' || escaped(text, j)) {
		j++
	}
	if j >= len(text) {
		return ""
	}
	if j+1 >= len(text) || text[j+1] != '(' {
		return ""
	}
	closeTarget := strings.IndexByte(text[j+2:], ')')
	if closeTarget < 0 {
		return ""
	}
	return cleanTarget(text[j+2 : j+2+closeTarget])
}

// escaped reports whether the byte at i follows an odd run of backslashes.
func escaped(s string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// cleanTarget reads the target out of a link's parentheses: no title, no
// fragment, and "" for schemes, absolute paths and bare fragments.
func cleanTarget(inside string) string {
	s := strings.TrimSpace(inside)
	if s == "" {
		return ""
	}
	if s[0] == '<' {
		close := strings.IndexByte(s, '>')
		if close < 0 {
			return ""
		}
		s = strings.TrimSpace(s[1:close])
	} else {
		s, _, _ = strings.Cut(s, " ")
	}
	if s == "" || s[0] == '#' || s[0] == '/' || hasScheme(s) {
		return ""
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	return s
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

// newline reads the line ending the file uses, LF when it has none.
func newline(src []byte) []byte {
	for i, b := range src {
		if b == '\n' {
			if i > 0 && src[i-1] == '\r' {
				return []byte("\r\n")
			}
			return []byte("\n")
		}
	}
	return []byte("\n")
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
