package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/store"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display knowledge base statistics, model info, and database size",
	Long:  `status displays counts of indexed chunks, files, entities, facts, identity fields, configured embedding model, and current database file size.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		out := cmd.OutOrStdout()

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		fi, err := os.Stat(dbPath)
		var dbSizeBytes int64
		if err == nil {
			dbSizeBytes = fi.Size()
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		var chunkCount, fileCount, entityCount, factCount, fieldCount int

		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM chunk").Scan(&chunkCount)
		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(DISTINCT path) FROM chunk").Scan(&fileCount)
		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM entity").Scan(&entityCount)
		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM fact").Scan(&factCount)
		_ = st.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM field").Scan(&fieldCount)

		client := embed.NewClient()

		fmt.Fprintln(out, "=== mini-me Knowledge Base Status ===")
		fmt.Fprintf(out, "Database Path:  %s\n", dbPath)
		fmt.Fprintf(out, "Database Size:  %.2f MB (%d bytes)\n", float64(dbSizeBytes)/(1024*1024), dbSizeBytes)
		fmt.Fprintf(out, "Embedding Model:%s:256 (Ollama %s)\n", client.ModelName(), client.BaseURL())
		fmt.Fprintln(out, "-------------------------------------")
		fmt.Fprintf(out, "Indexed Files:  %d\n", fileCount)
		fmt.Fprintf(out, "Text Chunks:    %d\n", chunkCount)
		fmt.Fprintf(out, "Entities:       %d\n", entityCount)
		fmt.Fprintf(out, "Facts:          %d\n", factCount)
		fmt.Fprintf(out, "Profile Fields: %d\n", fieldCount)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(statusCmd)
}
