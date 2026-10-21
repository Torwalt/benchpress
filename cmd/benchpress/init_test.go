package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	benchpress "github.com/Torwalt/benchpress"
	"github.com/Torwalt/benchpress/internal/config"
)

// initFiles are the pieces a fresh init writes.
var initFiles = []string{
	".benchpress.toml",
	"docs/workbench/NEXT.md",
	".agents/skills/benchpress/SKILL.md",
}

func TestInitCreatesWorkbench(t *testing.T) {
	root := t.TempDir()
	out, err := runRoot(t, root, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	for _, rel := range initFiles {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("init did not create %s: %v", rel, err)
		}
		if !strings.Contains(out, rel) {
			t.Errorf("init output %q does not name %s", out, rel)
		}
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if cfg.Dir != config.DefaultDir || !reflect.DeepEqual(cfg.Kinds, config.DefaultKinds) || len(cfg.Areas) != 0 {
		t.Errorf("written config = %+v, want the defaults", cfg)
	}
	if got := readAt(t, root, ".agents/skills/benchpress/SKILL.md"); got != benchpress.Skill {
		t.Errorf("installed skill differs from the embedded one")
	}
}

func TestInitFromChildUsesRoot(t *testing.T) {
	root := t.TempDir()
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	child := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runRoot(t, child, "init")
	if err != nil {
		t.Fatalf("init from child: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("init from child printed %q, want nothing", out)
	}
	for _, rel := range initFiles {
		if _, err := os.Lstat(filepath.Join(child, filepath.FromSlash(rel))); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("init from child created nested %s (err=%v)", rel, err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("init from child lost %s: %v", rel, err)
		}
	}
}

func TestInitHonorsConfiguredDirectory(t *testing.T) {
	root := t.TempDir()
	const cfg = "dir = \"custom\"\n"
	writeFile(t, root, ".benchpress.toml", cfg)

	out, err := runRoot(t, root, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if got := readAt(t, root, ".benchpress.toml"); got != cfg {
		t.Errorf("init changed the config: %q", got)
	}
	if got := readAt(t, root, "custom/NEXT.md"); got != "# Next\n\n" {
		t.Errorf("custom/NEXT.md = %q", got)
	}
	if !strings.Contains(out, "custom/NEXT.md") {
		t.Errorf("init output %q does not name custom/NEXT.md", out)
	}
	defaultNext := filepath.Join(config.DefaultDir, "NEXT.md")
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(defaultNext))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("init created the default-dir Next %s (err=%v)", defaultNext, err)
	}
}

func TestInitSecondRunChangesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	before := initSnapshot(t, root)
	link := filepath.Join(root, ".claude/skills/benchpress")
	beforeLink, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat installed link: %v", err)
	}
	out, err := runRoot(t, root, "init")
	if err != nil {
		t.Fatalf("second init: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("second init printed %q, want nothing", out)
	}
	if after := initSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("second init changed files:\nbefore=%v\nafter=%v", before, after)
	}
	afterLink, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat link after second init: %v", err)
	}
	if !os.SameFile(beforeLink, afterLink) {
		t.Error("second init replaced the Claude symlink")
	}
	if beforeLink.Mode() != afterLink.Mode() || !beforeLink.ModTime().Equal(afterLink.ModTime()) {
		t.Errorf("second init changed the Claude symlink's metadata:\nbefore=%v\nafter=%v", beforeLink, afterLink)
	}
}

func TestInitSkillRewrites(t *testing.T) {
	root := t.TempDir()
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	writeFile(t, root, ".agents/skills/benchpress/SKILL.md", "stale\n")
	writeFile(t, root, ".benchpress.toml", "dir = \"custom\"\n")

	if out, err := runRoot(t, root, "init", "--skill"); err != nil {
		t.Fatalf("init --skill: %v\n%s", err, out)
	}
	if got := readAt(t, root, ".agents/skills/benchpress/SKILL.md"); got != benchpress.Skill {
		t.Errorf("init --skill did not restore the skill:\n%s", got)
	}
	if got := readAt(t, root, ".benchpress.toml"); got != "dir = \"custom\"\n" {
		t.Errorf("init --skill overwrote the config: %q", got)
	}
}

func TestInitClaudeSymlink(t *testing.T) {
	without := t.TempDir()
	if out, err := runRoot(t, without, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if _, err := os.Lstat(filepath.Join(without, ".claude/skills/benchpress")); !os.IsNotExist(err) {
		t.Errorf("symlink exists without .claude/skills (err=%v)", err)
	}

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude/skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runRoot(t, root, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	link := filepath.Join(root, ".claude/skills/benchpress")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "../../.agents/skills/benchpress" {
		t.Errorf("symlink target = %q", target)
	}
	if data, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err != nil {
		t.Errorf("read through symlink: %v", err)
	} else if string(data) != benchpress.Skill {
		t.Errorf("symlink does not resolve to the skill")
	}
}

func TestInitClaudeSkillsStatError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("skills", filepath.Join(root, ".claude/skills")); err != nil {
		t.Fatalf("symlink loop: %v", err)
	}

	out, err := runRoot(t, root, "init")
	if err == nil {
		t.Fatalf("init succeeded despite the .claude/skills stat error\n%s", out)
	}
}

func TestSkillCmdPrintsEmbedded(t *testing.T) {
	root := t.TempDir()
	out, err := runRoot(t, root, "skill")
	if err != nil {
		t.Fatalf("skill: %v\n%s", err, out)
	}
	if out != benchpress.Skill {
		t.Errorf("skill output differs from the embedded skill")
	}
}

var (
	errInjectedWrite = errors.New("injected write failure")
	errInjectedClose = errors.New("injected close failure")
)

// injectWriteFailure writes n bytes and then fails until the returned restore
// runs. The cleanup keeps a failed test from leaking the seam.
func injectWriteFailure(t *testing.T, n int) (restore func()) {
	t.Helper()
	original := writeData
	limit := n
	writeData = func(f *os.File, data []byte) error {
		if limit > len(data) {
			limit = len(data)
		}
		if _, err := f.Write(data[:limit]); err != nil {
			return err
		}
		return errInjectedWrite
	}
	restore = func() { writeData = original }
	t.Cleanup(restore)
	return restore
}

// injectCloseFailure closes the file and then reports a close failure.
func injectCloseFailure(t *testing.T) (restore func()) {
	t.Helper()
	original := closeFile
	closeFile = func(f *os.File) error {
		_ = f.Close()
		return errInjectedClose
	}
	restore = func() { closeFile = original }
	t.Cleanup(restore)
	return restore
}

func TestCreateOnlyWriteFailureLeavesNoTarget(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "SKILL.md")
	restore := injectWriteFailure(t, 8)

	ok, err := createOnly(name, []byte(benchpress.Skill))
	if err == nil {
		t.Fatal("createOnly succeeded, want the injected write failure")
	}
	if ok {
		t.Error("createOnly reported a write")
	}
	if _, err := os.Stat(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("partial target left behind: %v", err)
	}

	restore()
	if ok, err := createOnly(name, []byte(benchpress.Skill)); err != nil || !ok {
		t.Fatalf("retry createOnly = %v, %v", ok, err)
	}
	if got := readAt(t, root, "SKILL.md"); got != benchpress.Skill {
		t.Errorf("retry did not install the whole skill")
	}
}

func TestCreateOnlyCloseFailureLeavesNoTarget(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "SKILL.md")
	restore := injectCloseFailure(t)

	ok, err := createOnly(name, []byte(benchpress.Skill))
	if err == nil {
		t.Fatal("createOnly succeeded, want the injected close failure")
	}
	if ok {
		t.Error("createOnly reported a write")
	}
	if _, err := os.Stat(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("partial target left behind: %v", err)
	}

	restore()
	if ok, err := createOnly(name, []byte(benchpress.Skill)); err != nil || !ok {
		t.Fatalf("retry createOnly = %v, %v", ok, err)
	}
	if got := readAt(t, root, "SKILL.md"); got != benchpress.Skill {
		t.Errorf("retry did not install the whole skill")
	}
}

func TestCreateOnlyLeavesExistingFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "SKILL.md", "kept\n")

	ok, err := createOnly(filepath.Join(root, "SKILL.md"), []byte(benchpress.Skill))
	if err != nil {
		t.Fatalf("createOnly: %v", err)
	}
	if ok {
		t.Error("createOnly overwrote an existing file")
	}
	if got := readAt(t, root, "SKILL.md"); got != "kept\n" {
		t.Errorf("existing file changed: %q", got)
	}
}

func TestInitSkillRewriteFailurePreservesOldFile(t *testing.T) {
	const old = "previous skill\n"
	root := t.TempDir()
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	writeFile(t, root, ".agents/skills/benchpress/SKILL.md", old)
	skillPath := filepath.Join(root, ".agents/skills/benchpress/SKILL.md")
	if err := os.Chmod(skillPath, 0o600); err != nil {
		t.Fatal(err)
	}

	restore := injectWriteFailure(t, 8)
	if out, err := runRoot(t, root, "init", "--skill"); err == nil {
		t.Fatalf("init --skill succeeded despite the injected write failure\n%s", out)
	}
	assertSkill(t, root, old, 0o600)

	restore()
	if out, err := runRoot(t, root, "init", "--skill"); err != nil {
		t.Fatalf("retry init --skill: %v\n%s", err, out)
	}
	assertSkill(t, root, benchpress.Skill, 0o600)
}

func TestInitSkillRewriteCloseFailurePreservesOldFile(t *testing.T) {
	const old = "previous skill\n"
	root := t.TempDir()
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	writeFile(t, root, ".agents/skills/benchpress/SKILL.md", old)

	restore := injectCloseFailure(t)
	if out, err := runRoot(t, root, "init", "--skill"); err == nil {
		t.Fatalf("init --skill succeeded despite the injected close failure\n%s", out)
	}
	assertSkill(t, root, old, 0o644)

	restore()
	if out, err := runRoot(t, root, "init", "--skill"); err != nil {
		t.Fatalf("retry init --skill: %v\n%s", err, out)
	}
	assertSkill(t, root, benchpress.Skill, 0o644)
}

func assertSkill(t *testing.T, root, body string, mode fs.FileMode) {
	t.Helper()
	if got := readAt(t, root, ".agents/skills/benchpress/SKILL.md"); got != body {
		t.Errorf("skill = %q, want %q", got, body)
	}
	info, err := os.Stat(filepath.Join(root, ".agents/skills/benchpress/SKILL.md"))
	if err != nil {
		t.Fatalf("stat skill: %v", err)
	}
	if got := info.Mode().Perm(); got != mode {
		t.Errorf("skill mode = %v, want %v", got, mode)
	}
}

// initSnapshot reads the files init writes, and the Claude symlink's target
// when it is there.
func initSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	for _, rel := range initFiles {
		snap[rel] = readAt(t, root, rel)
	}
	if target, err := os.Readlink(filepath.Join(root, ".claude/skills/benchpress")); err == nil {
		snap[".claude/skills/benchpress"] = target
	}
	return snap
}
