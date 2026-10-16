package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Change the Next list",
	}
	cmd.AddCommand(newNextAddCmd(), newNextRmCmd(), newNextMvCmd())
	return cmd
}

func newNextAddCmd() *cobra.Command {
	var at int
	cmd := &cobra.Command{
		Use:   "add <slug>",
		Short: "Add a topic to Next, at the end or at a position",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			return b.NextAdd(args[0], at)
		},
	}
	cmd.Flags().IntVar(&at, "at", 0, "1-based position; 0 appends")
	return cmd
}

func newNextRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <slug>",
		Short: "Remove a topic from Next",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			return b.NextRemove(args[0])
		},
	}
	return cmd
}

func newNextMvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mv <slug> <position>",
		Short: "Move a topic within Next",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			at, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("position %q is not a number", args[1])
			}
			b, err := loadBoard()
			if err != nil {
				return err
			}
			return b.NextMove(args[0], at)
		},
	}
	return cmd
}
