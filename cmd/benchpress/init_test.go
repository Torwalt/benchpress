package main

import (
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

func TestInitSecondRunChangesNothing(t *testing.T) {
	root := t.TempDir()
	if out, err := runRoot(t, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	before := initSnapshot(t, root)
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
