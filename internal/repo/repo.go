// Package repo locates repository-relative paths in the working tree and in the
// trees of local branches. It only ever reads git.
package repo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// errNotRepository marks git's own diagnostic for a directory outside a
// repository. Only that diagnostic permits the worktree-only fallback.
var errNotRepository = errors.New("not a git repository")

// gitError keeps the command, git's stderr and the process error together.
type gitError struct {
	args    []string
	stderr  string
	err     error
	outside bool
}

func (e *gitError) Error() string {
	msg := fmt.Sprintf("git %s: %v", strings.Join(e.args, " "), e.err)
	if stderr := strings.TrimSpace(e.stderr); stderr != "" {
		msg += ": " + stderr
	}
	return msg
}

func (e *gitError) Unwrap() error { return e.err }

func (e *gitError) Is(target error) bool {
	return target == errNotRepository && e.outside
}

// withoutLocale drops an inherited LC_ALL so the C locale command's
// diagnostic is the one git prints.
func withoutLocale(env []string) []string {
	out := env[:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "LC_ALL=") {
			out = append(out, kv)
		}
	}
	return out
}

// Worktree names the working tree in Find's results.
const Worktree = "worktree"

// Locator reports where repository-relative paths are found. lint takes one so
// its tests need no git.
type Locator interface {
	// Find returns, for every path, the places holding it: Worktree when it
	// exists on disk, then each local branch whose tree holds it.
	Find(ctx context.Context, paths ...string) (map[string][]string, error)
}

// Repo locates paths under a git repository root.
type Repo struct {
	root string
	// run runs one git command; tests wrap it to record the commands.
	run func(ctx context.Context, args ...string) ([]byte, error)
}

var _ Locator = (*Repo)(nil)

// New returns a Locator for the repository rooted at root.
func New(root string) *Repo {
	return &Repo{
		root: root,
		run: func(ctx context.Context, args ...string) ([]byte, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			cmd := exec.CommandContext(ctx, "git", args...)
			cmd.Dir = root
			cmd.Env = append(withoutLocale(os.Environ()), "LC_ALL=C")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				return nil, &gitError{
					args:    args,
					stderr:  stderr.String(),
					err:     err,
					outside: strings.HasPrefix(stderr.String(), "fatal: not a git repository"),
				}
			}
			return out, nil
		},
	}
}

// Find asks git once per local branch, as docs/DESIGN.md §5 specifies: one
// for-each-ref for the branches, then one ls-tree per branch over every path.
// Outside a git repository only the working tree is consulted, without error.
func (r *Repo) Find(ctx context.Context, paths ...string) (map[string][]string, error) {
	found := make(map[string][]string, len(paths))
	unique := make([]string, 0, len(paths))
	for _, p := range paths {
		if _, seen := found[p]; seen {
			continue
		}
		found[p] = nil
		unique = append(unique, p)
	}
	if len(unique) == 0 {
		return found, nil
	}

	for _, p := range unique {
		if r.inWorktree(p) {
			found[p] = append(found[p], Worktree)
		}
	}

	branches, err := r.branches(ctx)
	if err != nil {
		if errors.Is(err, errNotRepository) {
			return found, nil
		}
		return found, err
	}
	for _, ref := range branches {
		held, err := r.branchHolds(ctx, ref, unique)
		if err != nil {
			return nil, err
		}
		branch := strings.TrimPrefix(ref, "refs/heads/")
		for _, p := range unique {
			if held[p] {
				found[p] = append(found[p], branch)
			}
		}
	}
	return found, nil
}

// inWorktree reports whether a repository-relative path exists on disk.
func (r *Repo) inWorktree(path string) bool {
	_, err := os.Stat(filepath.Join(r.root, filepath.FromSlash(path)))
	return err == nil
}

// branches lists the local branches as full refs, or an error outside a
// repository. Full refs keep ls-tree unambiguous when a tag shares a branch's
// name; Find strips the prefix only for display.
func (r *Repo) branches(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, "for-each-ref", "--format=%(refname)", "refs/heads")
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}

// branchHolds returns the subset of paths present in ref's tree. The -z output
// is literal, with no C quoting of unusual bytes.
func (r *Repo) branchHolds(ctx context.Context, ref string, paths []string) (map[string]bool, error) {
	args := make([]string, 0, 6+len(paths))
	args = append(args, "ls-tree", "-r", "-z", "--name-only", ref, "--")
	args = append(args, paths...)
	out, err := r.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	held := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\x00") {
		if line != "" {
			held[line] = true
		}
	}
	return held, nil
}

func nonEmptyLines(out []byte) []string {
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
