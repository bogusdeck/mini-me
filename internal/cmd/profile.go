package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"mini-me/internal/profile"
	"mini-me/internal/store"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Generate and display your deterministic profile card",
	Long:  `profile synthesizes a clean, deterministic Markdown profile card (<= ~1500 tokens) from your identity fields, entities, and confirmed facts.`,
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

		gen := profile.NewGenerator(st)
		cardMarkdown, err := gen.GenerateProfileCard(ctx)
		if err != nil {
			return fmt.Errorf("failed generating profile card: %w", err)
		}

		fmt.Print(cardMarkdown)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(profileCmd)
}
