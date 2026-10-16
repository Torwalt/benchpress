package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Torwalt/benchpress/internal/lint"
	"github.com/Torwalt/benchpress/internal/repo"
	"github.com/spf13/cobra"
)

func newLintCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Report every problem in the workbench",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			problems, err := lint.Check(cmd.Context(), b, repo.New(b.Root))
			if err != nil {
				return err
			}
			if asJSON {
				err = writeProblemsJSON(cmd.OutOrStdout(), problems)
			} else {
				writeProblems(cmd.OutOrStdout(), problems)
			}
			if err != nil {
				return err
			}
			if anyError(problems) {
				return exitError(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

// writeProblems prints one problem per line, as §6 specifies.
func writeProblems(w io.Writer, problems []lint.Problem) {
	for _, p := range problems {
		fmt.Fprintf(w, "%s:%d: %s: %s\n", p.Path, p.Line, p.Severity, p.Message)
	}
}

type jsonProblems struct {
	Problems []lint.Problem `json:"problems"`
}

func writeProblemsJSON(w io.Writer, problems []lint.Problem) error {
	if problems == nil {
		problems = []lint.Problem{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonProblems{Problems: problems})
}

func anyError(problems []lint.Problem) bool {
	for _, p := range problems {
		if p.Severity == lint.Error {
			return true
		}
	}
	return false
}
