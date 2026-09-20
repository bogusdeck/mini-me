package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "0.1.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of memex",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("memex v%s\n", Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
