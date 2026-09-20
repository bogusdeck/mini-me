package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/ingest"
	"mini-me/internal/store"
)

var flagProject string

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Ingest a markdown file or directory into mini-me",
	Long:  `add scans the given file or directory, parses markdown headings, performs secret scanning, computes content-addressed diffs, and indexes chunks into the local SQLite database.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		targetPath := args[0]

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		embedClient := embed.NewClient()
		ingester := ingest.NewIngester(st, embedClient)

		stats, err := ingester.IngestPath(ctx, targetPath, flagProject)
		if err != nil {
			return fmt.Errorf("ingestion failed: %w", err)
		}

		fmt.Printf("Ingestion completed:\n")
		fmt.Printf("  Files processed: %d\n", stats.FilesProcessed)
		fmt.Printf("  Chunks created:   %d\n", stats.ChunksCreated)
		fmt.Printf("  Chunks deleted:   %d\n", stats.ChunksDeleted)
		fmt.Printf("  Chunks unchanged: %d\n", stats.ChunksUnchanged)

		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&flagProject, "project", "p", "", "optional project moniker to associate with chunks")
	RootCmd.AddCommand(addCmd)
}
