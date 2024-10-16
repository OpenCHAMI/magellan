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
	Use:   "version",
	Short: "Print version info and exit",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("commit").Value.String() == "true" {
			output = commit
			if date != "" {
				output += " built on " + date
			}
			fmt.Println(output)
		} else {
			fmt.Printf("%s-%s\n", version, commit)
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
