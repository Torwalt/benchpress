package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNewCmd() *cobra.Command {
	var (
		kind   string
		area   string
		epic   string
		toNext bool
	)
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a topic file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			path, err := b.New(args[0], kind, area, epic, toNext)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "the topic's kind (required)")
	cmd.Flags().StringVar(&area, "area", "", "the topic's area (required)")
	cmd.Flags().StringVar(&epic, "epic", "", "the slug of the epic the topic belongs to")
	cmd.Flags().BoolVar(&toNext, "next", false, "also append the topic to Next")
	_ = cmd.MarkFlagRequired("kind")
	_ = cmd.MarkFlagRequired("area")
	return cmd
}
