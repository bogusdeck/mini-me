package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"mini-me/internal/config"
	"mini-me/internal/profile"
	"mini-me/internal/store"
)

var exportCmd = &cobra.Command{
	Use:   "export <output_dir>",
	Short: "Export knowledge base, sqlite copy, profile card, facts, and chunks",
	Long:  `export creates a full backup copy of mini-me database, profile card markdown, facts graph JSON, and chunks JSONLines into the designated output directory.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		outDir := args[0]

		if err := config.EnsureDir(outDir); err != nil {
			return fmt.Errorf("failed creating output directory: %w", err)
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

		// 1. Export SQLite Copy
		dbCopyPath := filepath.Join(outDir, "mini-me.db.copy")
		srcFile, err := os.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed opening source db for copy: %w", err)
		}
		dstFile, err := os.OpenFile(dbCopyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			_ = srcFile.Close()
			return fmt.Errorf("failed creating destination db copy: %w", err)
		}
		_, copyErr := io.Copy(dstFile, srcFile)
		_ = srcFile.Close()
		_ = dstFile.Close()
		if copyErr != nil {
			return fmt.Errorf("failed copying db file: %w", copyErr)
		}

		// 2. Export Profile Card Markdown
		gen := profile.NewGenerator(st)
		cardMarkdown, err := gen.GenerateProfileCard(ctx)
		if err != nil {
			return fmt.Errorf("failed generating profile card for export: %w", err)
		}
		if err := os.WriteFile(filepath.Join(outDir, "profile.md"), []byte(cardMarkdown), 0600); err != nil {
			return fmt.Errorf("failed writing profile.md: %w", err)
		}

		// 3. Export Facts & Entities JSON
		entities, _ := st.ListEntities(ctx)
		confirmedFacts, _ := st.ListFactsByStatus(ctx, store.FactStatusConfirmed)
		proposedFacts, _ := st.ListFactsByStatus(ctx, store.FactStatusProposed)

		factsExport := map[string]any{
			"entities":        entities,
			"confirmed_facts": confirmedFacts,
			"proposed_facts":  proposedFacts,
		}
		factsJSON, _ := json.MarshalIndent(factsExport, "", "  ")
		if err := os.WriteFile(filepath.Join(outDir, "facts.json"), factsJSON, 0600); err != nil {
			return fmt.Errorf("failed writing facts.json: %w", err)
		}

		// 4. Export Chunks JSONL
		chunksFile, err := os.OpenFile(filepath.Join(outDir, "chunks.jsonl"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("failed creating chunks.jsonl: %w", err)
		}

		rows, err := st.DB().QueryContext(ctx, "SELECT id, source_type, path, project, text, hash, ts, embed_model, chunker_version FROM chunk")
		if err == nil {
			for rows.Next() {
				c := &store.Chunk{}
				if err := rows.Scan(&c.ID, &c.SourceType, &c.Path, &c.Project, &c.Text, &c.Hash, &c.TS, &c.EmbedModel, &c.ChunkerVersion); err == nil {
					lineBytes, _ := json.Marshal(c)
					_, _ = fmt.Fprintf(chunksFile, "%s\n", lineBytes)
				}
			}
			_ = rows.Close()
		}
		_ = chunksFile.Close()

		fmt.Printf("[OK] Export completed successfully to %s\n", outDir)
		fmt.Printf("  - %s\n", dbCopyPath)
		fmt.Printf("  - %s\n", filepath.Join(outDir, "profile.md"))
		fmt.Printf("  - %s\n", filepath.Join(outDir, "facts.json"))
		fmt.Printf("  - %s\n", filepath.Join(outDir, "chunks.jsonl"))

		return nil
	},
}

func init() {
	RootCmd.AddCommand(exportCmd)
}
