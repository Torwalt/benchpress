// Package config loads .benchpress.toml and finds the repository root.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName marks a directory as a workbench root.
const FileName = ".benchpress.toml"

// DefaultDir is where topics live when dir is not configured.
const DefaultDir = "docs/workbench"

// DefaultKinds are the allowed kinds when kinds is not configured.
var DefaultKinds = []string{"bug", "change", "design", "idea"}

// ErrNoRoot reports that no .benchpress.toml was found at or above a directory.
var ErrNoRoot = errors.New("no .benchpress.toml here or above; run benchpress init")

// Config is a repository's workbench configuration.
type Config struct {
	Dir   string   `toml:"dir"`
	Kinds []string `toml:"kinds"`
	Areas []string `toml:"areas"`
}

// Load reads .benchpress.toml from root, applying defaults for absent keys.
func Load(root string) (*Config, error) {
	var cfg Config
	meta, err := toml.DecodeFile(filepath.Join(root, FileName), &cfg)
	if err != nil {
		return nil, err
	}
	if unknown := meta.Undecoded(); len(unknown) > 0 {
		keys := make([]string, len(unknown))
		for i, key := range unknown {
			keys[i] = `"` + key.String() + `"`
		}
		return nil, fmt.Errorf("unknown key %s in %s", strings.Join(keys, ", "), FileName)
	}
	if cfg.Dir == "" {
		cfg.Dir = DefaultDir
	}
	if !meta.IsDefined("kinds") {
		cfg.Kinds = append([]string(nil), DefaultKinds...)
	}
	return &cfg, nil
}

// FindRoot walks up from start to the first directory holding .benchpress.toml.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, FileName)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoRoot
		}
		dir = parent
	}
}
