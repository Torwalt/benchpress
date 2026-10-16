package main

import (
	"fmt"

	benchpress "github.com/Torwalt/benchpress"
	"github.com/spf13/cobra"
)

func newSkillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Print the embedded skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), benchpress.Skill)
			return err
		},
	}
}
