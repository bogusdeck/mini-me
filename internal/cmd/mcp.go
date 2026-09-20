package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"mini-me/internal/embed"
	"mini-me/internal/mcp"
	"mini-me/internal/store"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start stdio Model Context Protocol (MCP) server",
	Long:  `mcp starts a stdio MCP server exposing read-only tools and propose_fact (writing exclusively to status=proposed review queue).`,
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
		srv := mcp.NewServer(st, embedClient, os.Stdin, os.Stdout)

		return srv.Run(ctx)
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
