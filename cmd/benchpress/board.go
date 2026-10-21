package main

import (
	"context"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/Torwalt/benchpress/internal/lint"
	"github.com/Torwalt/benchpress/internal/repo"
	"github.com/Torwalt/benchpress/internal/tui"
)

// stdoutIsTerminal reports whether stdout is a character device, the board's
// condition of §7. A pipe or a file runs ls instead.
func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// locator answers lint from plan locations already found, so opening the board
// asks git once.
type locator map[string][]string

func (l locator) Find(context.Context, ...string) (map[string][]string, error) {
	return l, nil
}

// runBoard loads the workbench, checks it and opens the board, which runs until
// the user quits.
func runBoard(cmd *cobra.Command) error {
	b, err := loadBoard()
	if err != nil {
		return err
	}
	planFound, err := findPlans(cmd, b)
	if err != nil {
		return err
	}
	problems, err := lint.Check(cmd.Context(), b, b.Exists, locator(planFound))
	if err != nil {
		return err
	}
	model := tui.New(tui.Options{
		Board:     b,
		PlanFound: planFound,
		Problems:  problems,
		Locator:   repo.New(b.Root),
	})
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}
