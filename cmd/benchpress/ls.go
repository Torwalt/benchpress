package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Torwalt/benchpress/internal/board"
	"github.com/Torwalt/benchpress/internal/config"
	"github.com/Torwalt/benchpress/internal/repo"
	"github.com/spf13/cobra"
)

func newLSCmd() *cobra.Command {
	var (
		states []string
		kinds  []string
		areas  []string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List the topics: Next first, then the rest by area and state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := loadBoard()
			if err != nil {
				return err
			}
			filter := board.Filter{States: states, Kinds: kinds, Areas: areas}
			planFound, err := findPlans(cmd, b)
			if err != nil {
				return err
			}
			nextSlugs, entries := b.Listing(filter, planFound)
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), nextSlugs, entries)
			}
			writeText(cmd.OutOrStdout(), nextSlugs, entries)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&states, "state", nil, "only topics in this state (repeatable)")
	cmd.Flags().StringArrayVar(&kinds, "kind", nil, "only topics of this kind (repeatable)")
	cmd.Flags().StringArrayVar(&areas, "area", nil, "only topics in this area (repeatable)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

// loadBoard finds the repository root from the working directory and loads it.
func loadBoard() (*board.Board, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := config.FindRoot(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return board.Load(root, cfg)
}

// findPlans locates every planned topic's plan, asking git once.
func findPlans(cmd *cobra.Command, b *board.Board) (map[string][]string, error) {
	plans := b.Plans()
	if len(plans) == 0 {
		return nil, nil
	}
	return repo.New(b.Root).Find(cmd.Context(), plans...)
}

func writeText(w io.Writer, nextSlugs []string, entries []board.Entry) {
	n := len(nextSlugs)
	if n > 0 {
		fmt.Fprintln(w, "Next")
		for _, e := range entries[:n] {
			fmt.Fprintln(w, entryLine(e))
		}
	}
	area := ""
	for i, e := range entries[n:] {
		if i == 0 || e.Area != area {
			area = e.Area
			if n > 0 || i > 0 {
				fmt.Fprintln(w)
			}
			if area != "" {
				fmt.Fprintln(w, area)
			}
		}
		fmt.Fprintln(w, entryLine(e))
	}
}

func entryLine(e board.Entry) string {
	if e.Broken() {
		return "  " + e.Slug + "  " + e.Issue.Message
	}
	line := "  " + e.Slug + "  " + e.State + "  " + e.Kind + "  " + e.Title
	if e.State == "planned" && e.Plan != "" {
		line += "  " + e.Plan
		if len(e.PlanFound) > 0 {
			line += " (" + strings.Join(e.PlanFound, ", ") + ")"
		} else {
			line += " (not found)"
		}
	}
	return line
}

type jsonTopic struct {
	Slug      string   `json:"slug"`
	Path      string   `json:"path"`
	Title     string   `json:"title"`
	Kind      string   `json:"kind"`
	Area      string   `json:"area"`
	State     string   `json:"state"`
	Plan      string   `json:"plan"`
	PlanFound []string `json:"plan_found"`
}

type jsonListing struct {
	Next   []string    `json:"next"`
	Topics []jsonTopic `json:"topics"`
}

func writeJSON(w io.Writer, nextSlugs []string, entries []board.Entry) error {
	out := jsonListing{Next: nextSlugs, Topics: make([]jsonTopic, 0, len(entries))}
	for _, e := range entries {
		found := e.PlanFound
		if found == nil {
			found = []string{}
		}
		out.Topics = append(out.Topics, jsonTopic{
			Slug:      e.Slug,
			Path:      e.Path,
			Title:     e.Title,
			Kind:      e.Kind,
			Area:      e.Area,
			State:     e.State,
			Plan:      e.Plan,
			PlanFound: found,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
