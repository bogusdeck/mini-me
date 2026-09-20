package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mini-me/internal/store"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review queue of proposed facts",
	Long:  `review lists all proposed facts submitted by external tools (e.g. MCP) that are awaiting user confirmation or rejection.`,
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

		facts, err := st.ListFactsByStatus(ctx, store.FactStatusProposed)
		if err != nil {
			return fmt.Errorf("failed listing proposed facts: %w", err)
		}

		if len(facts) == 0 {
			fmt.Println("Review queue is empty. No proposed facts pending.")
			return nil
		}

		entities, _ := st.ListEntities(ctx)
		entMap := make(map[int64]string)
		for _, e := range entities {
			entMap[e.ID] = e.Name
		}

		fmt.Printf("=== Proposed Fact Review Queue (%d pending) ===\n\n", len(facts))

		for i, f := range facts {
			subjName := entMap[f.SubjectID]
			if subjName == "" {
				subjName = fmt.Sprintf("Subject #%d", f.SubjectID)
			}

			fmt.Printf("%d. Fact #%d:\n", i+1, f.ID)
			fmt.Printf("   Statement: %s %s %s\n", subjName, f.Predicate, f.Object)
			fmt.Printf("   Sensitivity: %s | Confidence: %.2f\n", f.Sensitivity, f.Confidence)

			// Query Evidence Chunk if linked
			rows, err := st.DB().QueryContext(ctx, `
				SELECT c.path, c.text FROM chunk c
				JOIN fact_source fs ON c.id = fs.chunk_id
				WHERE fs.fact_id = ?
			`, f.ID)
			if err == nil {
				var count int
				for rows.Next() {
					var p, text string
					if err := rows.Scan(&p, &text); err == nil {
						count++
						fmt.Printf("   Evidence source #%d: %s\n", count, p)
					}
				}
				_ = rows.Close()
			}

			fmt.Printf("   To confirm: `mini-me fact confirm %d` | To reject: `mini-me fact reject %d`\n\n", f.ID, f.ID)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(reviewCmd)
}
