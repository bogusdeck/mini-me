package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/store"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run system health diagnostics for mini-me",
	Long:  `doctor checks local SQLite storage accessibility, Ollama server reachability, and verifies that the embedding model (nomic-embed-text) is installed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		out := cmd.OutOrStdout()

		fmt.Fprintln(out, "Running mini-me diagnostics...")
		fmt.Fprintln(out, "---------------------------------")

		allPassed := true

		// 1. DB Accessibility Check
		dbPath, err := GetDBPath()
		if err != nil {
			fmt.Fprintf(out, "[FAIL] Database path error: %v\n", err)
			allPassed = false
		} else {
			st, err := store.Open(ctx, dbPath)
			if err != nil {
				fmt.Fprintf(out, "[FAIL] SQLite DB error at %s: %v\n", dbPath, err)
				allPassed = false
			} else {
				_ = st.Close()
				fmt.Fprintf(out, "[OK] SQLite database accessible at %s\n", dbPath)
			}
		}

		// 2. Ollama Reachability Check
		client := embed.NewClient()
		if err := client.CheckReachability(ctx); err != nil {
			fmt.Fprintf(out, "[FAIL] Ollama is unreachable at %s\n", client.BaseURL())
			fmt.Fprintln(out, "       -> Please ensure Ollama is installed and running.")
			fmt.Fprintln(out, "       -> Install / Start Ollama: https://ollama.com")
			allPassed = false
		} else {
			fmt.Fprintf(out, "[OK] Ollama server is reachable at %s\n", client.BaseURL())

			// 3. Model Pulled Check
			pulled, err := client.CheckModelPulled(ctx)
			if err != nil {
				fmt.Fprintf(out, "[FAIL] Failed checking Ollama models: %v\n", err)
				allPassed = false
			} else if !pulled {
				fmt.Fprintf(out, "[FAIL] Embedding model %q is not installed in Ollama.\n", client.ModelName())
				fmt.Fprintf(out, "       -> Please run: ollama pull %s\n", client.ModelName())
				allPassed = false
			} else {
				fmt.Fprintf(out, "[OK] Embedding model %q is pulled and ready.\n", client.ModelName())
			}
		}

		fmt.Fprintln(out, "---------------------------------")
		if !allPassed {
			fmt.Fprintln(out, "Diagnostics completed with issues. Please follow the instructions above.")
			os.Exit(1)
		}

		fmt.Fprintln(out, "All diagnostics passed successfully!")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}
