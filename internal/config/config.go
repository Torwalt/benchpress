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

// knownKeys are the only top-level keys .benchpress.toml accepts.
var knownKeys = map[string]bool{"dir": true, "kinds": true, "areas": true}

// Load reads .benchpress.toml from root, applying defaults for absent keys.
func Load(root string) (*Config, error) {
	var cfg Config
	meta, err := toml.DecodeFile(filepath.Join(root, FileName), &cfg)
	if err != nil {
		return nil, err
	}
	// Decoding matches fields case-insensitively, so check the key spellings too.
	var unknown []string
	for _, key := range meta.Keys() {
		if len(key) != 1 || !knownKeys[key[0]] {
			unknown = append(unknown, `"`+key.String()+`"`)
		}
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("unknown key %s in %s", strings.Join(unknown, ", "), FileName)
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
		// Stat follows symlinks, so a link to a regular file counts too.
		if info, err := os.Stat(filepath.Join(dir, FileName)); err == nil && info.Mode().IsRegular() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNoRoot
		}
		dir = parent
	}
}
