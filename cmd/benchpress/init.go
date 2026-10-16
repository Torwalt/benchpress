package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	benchpress "github.com/Torwalt/benchpress"
	"github.com/Torwalt/benchpress/internal/config"
	"github.com/spf13/cobra"
)

// Where init installs the skill: the shared agents directory, and the Claude
// symlink that points at it when the repository keeps one.
const (
	skillFile    = ".agents/skills/benchpress/SKILL.md"
	skillDir     = ".agents/skills/benchpress"
	claudeSkills = ".claude/skills"
	claudeLink   = ".claude/skills/benchpress"
)

func newInitCmd() *cobra.Command {
	var rewriteSkill bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the workbench and install the skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := initRoot()
			if err != nil {
				return err
			}
			created, err := installWorkbench(root, rewriteSkill)
			if err != nil {
				return err
			}
			for _, rel := range created {
				fmt.Fprintln(cmd.OutOrStdout(), rel)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&rewriteSkill, "skill", false, "rewrite the installed skill from the binary")
	return cmd
}

// initRoot finds the repository root, falling back to the working directory
// when none is initialised yet.
func initRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := config.FindRoot(cwd)
	if errors.Is(err, config.ErrNoRoot) {
		return cwd, nil
	}
	if err != nil {
		return "", err
	}
	return root, nil
}

// installWorkbench creates the missing pieces of a workbench at root and
// returns the paths it wrote. It never overwrites a file, except the skill when
// rewriteSkill is set.
func installWorkbench(root string, rewriteSkill bool) ([]string, error) {
	cfg, err := existingOrDefaultConfig(root)
	if err != nil {
		return nil, err
	}
	var created []string
	add := func(rel string) { created = append(created, filepath.ToSlash(rel)) }

	if ok, err := createOnly(filepath.Join(root, config.FileName), defaultConfigFile()); err != nil {
		return nil, err
	} else if ok {
		add(config.FileName)
	}

	next := path.Join(cfg.Dir, "NEXT.md")
	if ok, err := createOnly(filepath.Join(root, filepath.FromSlash(next)), []byte("# Next\n\n")); err != nil {
		return nil, err
	} else if ok {
		add(next)
	}

	fullSkill := filepath.Join(root, filepath.FromSlash(skillFile))
	if rewriteSkill {
		if err := os.MkdirAll(filepath.Dir(fullSkill), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(fullSkill, []byte(benchpress.Skill), 0o644); err != nil {
			return nil, err
		}
		add(skillFile)
	} else if ok, err := createOnly(fullSkill, []byte(benchpress.Skill)); err != nil {
		return nil, err
	} else if ok {
		add(skillFile)
	}

	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(claudeSkills))); err == nil && info.IsDir() {
		link := filepath.Join(root, filepath.FromSlash(claudeLink))
		if ok, err := symlinkOnly(link, "../../"+skillDir); err != nil {
			return nil, err
		} else if ok {
			add(claudeLink)
		}
	}
	return created, nil
}

// existingOrDefaultConfig reads the workbench's config when it exists, so init
// completes an existing workbench in its configured directory.
func existingOrDefaultConfig(root string) (*config.Config, error) {
	if _, err := os.Stat(filepath.Join(root, config.FileName)); err == nil {
		return config.Load(root)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return &config.Config{Dir: config.DefaultDir, Kinds: append([]string(nil), config.DefaultKinds...)}, nil
}

// defaultConfigFile is .benchpress.toml with only the keys that have defaults:
// areas omitted means any slug is allowed (§2).
func defaultConfigFile() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "dir = %q\n", config.DefaultDir)
	b.WriteString("kinds = [")
	for i, kind := range config.DefaultKinds {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", kind)
	}
	b.WriteString("]\n")
	return []byte(b.String())
}

// createOnly writes data to name unless it already exists, reporting whether it
// wrote. O_EXCL makes the check and the create one step.
func createOnly(name string, data []byte) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	return true, nil
}

// symlinkOnly creates a symlink unless something already stands at its path.
func symlinkOnly(name, target string) (bool, error) {
	switch _, err := os.Lstat(name); {
	case err == nil:
		return false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return false, err
	}
	if err := os.Symlink(target, name); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
