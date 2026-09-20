package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/ingest"
	"mini-me/internal/store"
)

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Reindex all previously added paths and purge deleted files",
	Long:  `reindex iterates over all file paths stored in mini-me, updates modified chunks, and removes chunks for files that have been deleted or filtered out.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

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

		fmt.Println("Reindexing knowledge base...")
		stats, err := ingester.Reindex(ctx)
		if err != nil {
			return fmt.Errorf("reindex failed: %w", err)
		}

		fmt.Printf("Reindex completed:\n")
		fmt.Printf("  Files processed: %d\n", stats.FilesProcessed)
		fmt.Printf("  Chunks created:   %d\n", stats.ChunksCreated)
		fmt.Printf("  Chunks deleted:   %d\n", stats.ChunksDeleted)
		fmt.Printf("  Chunks unchanged: %d\n", stats.ChunksUnchanged)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(reindexCmd)
}
