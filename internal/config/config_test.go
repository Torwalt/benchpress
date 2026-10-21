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

func TestLoadRejectsCaseVariantKeys(t *testing.T) {
	for _, tc := range []struct {
		key  string
		body string
	}{
		{"Kinds", "Kinds = [\"bug\"]\n"},
		{"Dir", "Dir = \"work\"\n"},
		{"AREAS", "AREAS = [\"maps\"]\n"},
		{"foo", "foo = \"bar\"\n"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, tc.body)

			_, err := Load(dir)
			if err == nil {
				t.Fatalf("Load: want error for key %q", tc.key)
			}
			if !strings.Contains(err.Error(), `"`+tc.key+`"`) {
				t.Errorf("error = %q, want it to name %q", err, tc.key)
			}
		})
	}

	t.Run("lowercase", func(t *testing.T) {
		dir := t.TempDir()
		writeConfig(t, dir, `dir = "work"
kinds = ["bug"]
areas = ["maps"]
`)

		cfg, err := Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Dir != "work" {
			t.Errorf("Dir = %q, want %q", cfg.Dir, "work")
		}
		if want := []string{"bug"}; !slices.Equal(cfg.Kinds, want) {
			t.Errorf("Kinds = %v, want %v", cfg.Kinds, want)
		}
		if want := []string{"maps"}; !slices.Equal(cfg.Areas, want) {
			t.Errorf("Areas = %v, want %v", cfg.Areas, want)
		}
	})
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

func TestFindRootSkipsConfigDirectory(t *testing.T) {
	t.Run("file above", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "")

		nested := filepath.Join(root, "a", "b")
		if err := os.MkdirAll(filepath.Join(nested, FileName), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		got, err := FindRoot(nested)
		if err != nil {
			t.Fatalf("FindRoot: %v", err)
		}
		if got != root {
			t.Errorf("FindRoot = %q, want %q", got, root)
		}
	})

	t.Run("none above", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, FileName), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}

		_, err := FindRoot(dir)
		if !errors.Is(err, ErrNoRoot) {
			t.Fatalf("FindRoot error = %v, want ErrNoRoot", err)
		}
	})
}

func TestFindRootFollowsConfigSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.toml")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, FileName)); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	nested := filepath.Join(root, "a")
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
