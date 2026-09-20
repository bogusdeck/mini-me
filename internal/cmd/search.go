package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/search"
	"mini-me/internal/store"
)

var (
	flagSearchSource  string
	flagSearchProject string
	flagSearchSince   string
	flagSearchK       int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Perform hybrid vector + keyword search across knowledge base",
	Long:  `search performs hybrid Vector KNN and FTS5 BM25 search, fused via Reciprocal Rank Fusion (RRF). Validates source file freshness before returning hits.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		queryStr := args[0]

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		var sinceTime time.Time
		if flagSearchSince != "" {
			if dur, err := time.ParseDuration(flagSearchSince); err == nil {
				sinceTime = time.Now().Add(-dur)
			} else if t, err := time.Parse("2006-01-02", flagSearchSince); err == nil {
				sinceTime = t
			} else {
				return fmt.Errorf("invalid --since format %q (use e.g. 24h, 7d, 2026-01-01)", flagSearchSince)
			}
		}

		embedClient := embed.NewClient()
		engine := search.NewSearchEngine(st, embedClient)

		opts := search.SearchOptions{
			SourceType: flagSearchSource,
			Project:    flagSearchProject,
			Since:      sinceTime,
			K:          flagSearchK,
		}

		hits, err := engine.Search(ctx, queryStr, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(hits) == 0 {
			fmt.Println("No matching search results found.")
			return nil
		}

		fmt.Printf("Search results for %q (%d hits):\n\n", queryStr, len(hits))

		for i, hit := range hits {
			projStr := ""
			if hit.Chunk.Project != "" {
				projStr = fmt.Sprintf("[%s] ", hit.Chunk.Project)
			}
			fmt.Printf("%d. %s%s (Score: %.4f)\n", i+1, projStr, hit.Chunk.Path, hit.RRFScore)
			fmt.Printf("   %s\n\n", hit.Chunk.Text)
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().StringVar(&flagSearchSource, "source", "", "filter search by source_type (e.g. markdown)")
	searchCmd.Flags().StringVar(&flagSearchProject, "project", "", "filter search by project moniker")
	searchCmd.Flags().StringVar(&flagSearchSince, "since", "", "filter search by cutoff time (e.g. 24h, 7d, 2026-01-01)")
	searchCmd.Flags().IntVarP(&flagSearchK, "k", "k", 10, "number of search results to return")
	RootCmd.AddCommand(searchCmd)
}
