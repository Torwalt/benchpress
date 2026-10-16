package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <slug>",
		Short: "Delete a topic file and remove it from Next",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			linked, err := b.Remove(args[0])
			if err != nil {
				return err
			}
			for _, p := range linked {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
	return cmd
}
