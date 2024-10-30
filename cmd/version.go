package cmd

import (
	"github.com/OpenCHAMI/magellan/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info and exit",
	Run: func(cmd *cobra.Command, args []string) {
		version.PrintVersionInfo()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
