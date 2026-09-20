package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"memex/internal/config"
	"memex/internal/logger"
)

var (
	flagDebug  bool
	flagDBPath string
)

// RootCmd represents the base command when called without subcommands.
var RootCmd = &cobra.Command{
	Use:   "memex",
	Short: "memex is a local-first personal knowledge base",
	Long: `memex is a local-first, single-user personal knowledge base for macOS and Linux.
It provides CLI tools, HTTP endpoints, and an MCP server for profile, project, and note management.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logger.Init(flagDebug)

		if flagDBPath != "" {
			_ = os.Setenv(config.EnvDBPath, flagDBPath)
		}

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	RootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "enable debug logging")
	RootCmd.PersistentFlags().StringVar(&flagDBPath, "db", "", "path to sqlite database file (overrides MEMEX_DB_PATH)")
}

func GetDBPath() (string, error) {
	return config.DBPath()
}
