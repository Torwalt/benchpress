// Command benchpress keeps a repository's workbench: one Markdown file per
// topic, an ordered Next list, a lint over both and a board to look at them.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is stamped at build time.
var version = "dev"

// exitError is a command's request for a process exit code, returned after the
// command has printed everything it has to say.
type exitError int

func (e exitError) Error() string { return "" }

func main() {
	err := newRootCmd().Execute()
	if err == nil {
		return
	}
	var code exitError
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	fmt.Fprintln(os.Stderr, "benchpress:", err)
	os.Exit(1)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "benchpress",
		Short:         "A workbench of topics as Markdown files, with a board over them",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newLSCmd())
	root.AddCommand(newLintCmd())
	return root
}
