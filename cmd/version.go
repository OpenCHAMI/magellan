package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
	output  string
)

var versionCmd = &cobra.Command{
	Use: "version",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("commit").Value.String() == "true" {
			output = commit
			if date != "" {
				output += " built @ " + date
			}
			fmt.Println(output)
		} else {
			fmt.Println(version)
		}
	},
}

func init() {
	versionCmd.Flags().Bool("commit", false, "show the version commit")
	rootCmd.AddCommand(versionCmd)
}

func SetVersionInfo(buildVersion string, buildCommit string, buildDate string) {
	version = buildVersion
	commit = buildCommit
	date = buildDate
}
