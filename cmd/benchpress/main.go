// Command benchpress keeps a repository's workbench: one Markdown file per
// topic, an ordered Next list, a lint over both and a board to look at them.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is stamped at build time.
var version = "dev"

func main() {
	root := &cobra.Command{
		Use:           "benchpress",
		Short:         "A workbench of topics as Markdown files, with a board over them",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "benchpress:", err)
		os.Exit(1)
	}
}
