package repo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// gitRepo makes a temporary git repository with a main branch and an identity,
// and points HOME at a temporary directory so no global config leaks in.
func gitRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func commit(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", message)
}

func find(t *testing.T, r *Repo, paths ...string) map[string][]string {
	t.Helper()
	got, err := r.Find(context.Background(), paths...)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	return got
}

func TestFindWorktreeOnly(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "docs/plans/loose.md", "x")

	got := find(t, New(dir), "docs/plans/loose.md")
	if want := []string{Worktree}; !reflect.DeepEqual(got["docs/plans/loose.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/loose.md"], want)
	}
}

func TestFindSecondBranchOnly(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "seed")
	commit(t, dir, "seed")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	writeFile(t, dir, "docs/plans/on-other.md", "x")
	commit(t, dir, "plan on other")
	runGit(t, dir, "checkout", "-q", "main")

	got := find(t, New(dir), "docs/plans/on-other.md")
	if want := []string{"other"}; !reflect.DeepEqual(got["docs/plans/on-other.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/on-other.md"], want)
	}
}

func TestFindWorktreeAndBranch(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "docs/plans/both.md", "x")
	commit(t, dir, "both")

	got := find(t, New(dir), "docs/plans/both.md")
	if want := []string{Worktree, "main"}; !reflect.DeepEqual(got["docs/plans/both.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/both.md"], want)
	}
}

func TestFindNowhere(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "seed")
	commit(t, dir, "seed")

	got := find(t, New(dir), "docs/plans/missing.md")
	if locations := got["docs/plans/missing.md"]; len(locations) != 0 {
		t.Errorf("locations = %v, want none", locations)
	}
}

func TestFindOutsideRepository(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	writeFile(t, dir, "docs/plans/here.md", "x")

	got := find(t, New(dir), "docs/plans/here.md")
	if want := []string{Worktree}; !reflect.DeepEqual(got["docs/plans/here.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/here.md"], want)
	}
}

func TestFindNonASCIIPlanOnBranch(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "seed")
	commit(t, dir, "seed")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	writeFile(t, dir, "docs/plans/café.md", "x")
	commit(t, dir, "plan on other")
	runGit(t, dir, "checkout", "-q", "main")

	got := find(t, New(dir), "docs/plans/café.md")
	if want := []string{"other"}; !reflect.DeepEqual(got["docs/plans/café.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/café.md"], want)
	}
}

func TestFindBranchNameWithTagCollision(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "README.md", "seed")
	commit(t, dir, "seed")
	runGit(t, dir, "checkout", "-q", "-b", "foo")
	writeFile(t, dir, "docs/plans/colliding.md", "x")
	commit(t, dir, "plan on foo")
	runGit(t, dir, "checkout", "-q", "main")
	runGit(t, dir, "branch", "heads/foo")
	runGit(t, dir, "tag", "foo")

	got := find(t, New(dir), "docs/plans/colliding.md")
	if want := []string{"foo"}; !reflect.DeepEqual(got["docs/plans/colliding.md"], want) {
		t.Errorf("locations = %v, want %v", got["docs/plans/colliding.md"], want)
	}
}

func TestFindRunsOneLsTreePerBranch(t *testing.T) {
	dir := gitRepo(t)
	writeFile(t, dir, "docs/plans/a.md", "x")
	commit(t, dir, "a")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	writeFile(t, dir, "docs/plans/b.md", "y")
	commit(t, dir, "b")
	runGit(t, dir, "checkout", "-q", "main")

	r := New(dir)
	inner := r.run
	var commands []string
	r.run = func(ctx context.Context, args ...string) ([]byte, error) {
		commands = append(commands, args[0])
		return inner(ctx, args...)
	}

	find(t, r, "docs/plans/a.md", "docs/plans/b.md")

	var refs, trees int
	for _, name := range commands {
		switch name {
		case "for-each-ref":
			refs++
		case "ls-tree":
			trees++
		default:
			t.Errorf("unexpected git command %q", name)
		}
	}
	if refs != 1 {
		t.Errorf("for-each-ref calls = %d, want 1", refs)
	}
	if trees != 2 {
		t.Errorf("ls-tree calls = %d, want one per branch (2)", trees)
	}
}
