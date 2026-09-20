package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"mini-me/internal/store"
)

var (
	flagForgetPath   string
	flagForgetSource string
	flagForgetSince  string
	flagForgetFile   string
	flagForgetAll    bool
)

var forgetCmd = &cobra.Command{
	Use:   "forget",
	Short: "Purge chunks and data from the knowledge base by path, source type, or date",
	Long:  `forget purges stored chunks, vectors, and search indices matching a specific path, source type, or modified date range.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if flagForgetPath == "" && flagForgetFile != "" {
			flagForgetPath = flagForgetFile
		}

		if flagForgetPath == "" && flagForgetSource == "" && flagForgetSince == "" && !flagForgetAll {
			return fmt.Errorf("specify at least one purge criteria: --path, --source, --since, or --all")
		}

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		if flagForgetAll {
			_, _ = st.DB().ExecContext(ctx, "DELETE FROM chunk_vec")
			_, _ = st.DB().ExecContext(ctx, "DELETE FROM chunk_fts")
			_, _ = st.DB().ExecContext(ctx, "DELETE FROM chunk")
			fmt.Println("[OK] All knowledge base chunks purged successfully.")
			return nil
		}

		var sinceTime time.Time
		if flagForgetSince != "" {
			if dur, err := time.ParseDuration(flagForgetSince); err == nil {
				sinceTime = time.Now().Add(-dur)
			} else if t, err := time.Parse("2006-01-02", flagForgetSince); err == nil {
				sinceTime = t
			} else {
				return fmt.Errorf("invalid --since format %q (use e.g. 24h, 7d, 2026-01-01)", flagForgetSince)
			}
		}

		// Find chunk IDs matching purge criteria
		query := "SELECT id, path FROM chunk WHERE 1=1"
		var queryArgs []any

		if flagForgetPath != "" {
			query += " AND path = ?"
			queryArgs = append(queryArgs, flagForgetPath)
		}
		if flagForgetSource != "" {
			query += " AND source_type = ?"
			queryArgs = append(queryArgs, flagForgetSource)
		}
		if !sinceTime.IsZero() {
			query += " AND ts >= ?"
			queryArgs = append(queryArgs, sinceTime.Unix())
		}

		rows, err := st.DB().QueryContext(ctx, query, queryArgs...)
		if err != nil {
			return fmt.Errorf("failed querying chunks to forget: %w", err)
		}

		var ids []int64
		for rows.Next() {
			var id int64
			var p string
			if err := rows.Scan(&id, &p); err == nil {
				ids = append(ids, id)
			}
		}
		_ = rows.Close()

		if len(ids) == 0 {
			fmt.Println("No matching chunks found to forget.")
			return nil
		}

		for _, id := range ids {
			if err := st.DeleteChunk(ctx, id); err != nil {
				return fmt.Errorf("failed deleting chunk #%d: %w", id, err)
			}
		}

		fmt.Printf("[OK] Purged %d chunk(s) from knowledge base.\n", len(ids))
		return nil
	},
}

func init() {
	forgetCmd.Flags().StringVar(&flagForgetPath, "path", "", "purge chunks matching exact file path")
	forgetCmd.Flags().StringVar(&flagForgetSource, "source", "", "purge chunks matching source_type (e.g. markdown)")
	forgetCmd.Flags().StringVar(&flagForgetSince, "since", "", "purge chunks added/modified since time (e.g. 24h, 7d)")
	forgetCmd.Flags().BoolVar(&flagForgetAll, "all", false, "purge all chunks in knowledge base")
	RootCmd.AddCommand(forgetCmd)
}
