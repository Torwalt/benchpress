package main

import "github.com/spf13/cobra"

func newStateCmd() *cobra.Command {
	var plan string
	cmd := &cobra.Command{
		Use:   "state <slug> <state>",
		Short: "Set a topic's state",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			return b.SetState(args[0], args[1], plan)
		},
	}
	cmd.Flags().StringVar(&plan, "plan", "", "the plan path, required for state planned")
	return cmd
}
