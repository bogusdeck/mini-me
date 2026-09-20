package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/ingest"
	"mini-me/internal/store"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan laptop data (browser history, desktop folders, identity documents, resumes, projects)",
	Long:  `scan analyzes browser history, local document folders, project repositories, resumes, and identity documents (PAN card, Aadhaar, Passport, Photos, Signatures), storing their locations and facts in mini-me.`,
}

var scanBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Scan Chrome and Safari browser history",
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
		bScanner := ingest.NewBrowserScanner(st, embedClient)

		fmt.Println("Scanning Chrome and Safari browser history...")
		count, err := bScanner.IngestBrowserHistory(ctx)
		if err != nil {
			return fmt.Errorf("browser history scan failed: %w", err)
		}

		fmt.Printf("[OK] Browser scan complete: indexed %d history activity chunk(s).\n", count)
		return nil
	},
}

var scanDocsCmd = &cobra.Command{
	Use:   "docs [target_path]",
	Short: "Scan user folders for resumes, projects, and personal identity documents",
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
		dScanner := ingest.NewDeepScanner(st, embedClient)

		var targetPaths []string
		if len(args) > 0 {
			targetPaths = append(targetPaths, args[0])
		}

		fmt.Println("Scanning local directories for resumes, projects, and identity documents...")
		stats, err := dScanner.ScanUserDirectories(ctx, targetPaths...)
		if err != nil {
			return fmt.Errorf("document scan failed: %w", err)
		}

		fmt.Printf("[OK] Deep Document Scan completed:\n")
		fmt.Printf("  Files scanned:       %d\n", stats.FilesScanned)
		fmt.Printf("  Resumes flagged:     %d\n", stats.ResumesFound)
		fmt.Printf("  Projects discovered: %d\n", stats.ProjectsFound)
		fmt.Printf("  Identity docs found: %d\n", stats.IdentityDocsFound)
		fmt.Printf("  Chunks created:      %d\n", stats.ChunksCreated)

		return nil
	},
}

var scanAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Scan browser history, documents, projects, and identity files",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := scanBrowserCmd.RunE(cmd, args); err != nil {
			fmt.Printf("Warning: browser scan error: %v\n", err)
		}
		if err := scanDocsCmd.RunE(cmd, args); err != nil {
			fmt.Printf("Warning: docs scan error: %v\n", err)
		}
		return nil
	},
}

func init() {
	scanCmd.AddCommand(scanBrowserCmd, scanDocsCmd, scanAllCmd)
	RootCmd.AddCommand(scanCmd)
}
