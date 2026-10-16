package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Dir != DefaultDir {
		t.Errorf("Dir = %q, want %q", cfg.Dir, DefaultDir)
	}
	if !slices.Equal(cfg.Kinds, DefaultKinds) {
		t.Errorf("Kinds = %v, want %v", cfg.Kinds, DefaultKinds)
	}
	if len(cfg.Areas) != 0 {
		t.Errorf("Areas = %v, want empty", cfg.Areas)
	}
}

func TestLoadFull(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `dir = "work"
kinds = ["bug", "idea"]
areas = ["maps", "army"]
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Dir != "work" {
		t.Errorf("Dir = %q, want %q", cfg.Dir, "work")
	}
	if want := []string{"bug", "idea"}; !slices.Equal(cfg.Kinds, want) {
		t.Errorf("Kinds = %v, want %v", cfg.Kinds, want)
	}
	if want := []string{"maps", "army"}; !slices.Equal(cfg.Areas, want) {
		t.Errorf("Areas = %v, want %v", cfg.Areas, want)
	}
}

func TestLoadUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `dir = "work"
foo = "bar"
`)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load: want error for unknown key")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Errorf("error = %q, want it to name the unknown key", err)
	}
}

func TestFindRootNested(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "")

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if got != root {
		t.Errorf("FindRoot = %q, want %q", got, root)
	}
}

func TestFindRootNone(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := FindRoot(nested)
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("FindRoot error = %v, want ErrNoRoot", err)
	}
	if err.Error() != "no .benchpress.toml here or above; run benchpress init" {
		t.Errorf("message = %q", err.Error())
	}
}
