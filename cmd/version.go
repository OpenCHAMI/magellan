package cmd

import (
	"fmt"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("rev").Value.String() == "true" {
			fmt.Println(magellan.VersionCommit())
		} else {
			fmt.Println(magellan.VersionTag())
		}
	},
}

func init() {
	versionCmd.Flags().Bool("rev", false, "show the version commit")
	rootCmd.AddCommand(versionCmd)
}
