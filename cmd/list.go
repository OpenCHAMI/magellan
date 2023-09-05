package cmd

import (
	"davidallendj/magellan/internal/db/sqlite"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)


var listCmd = &cobra.Command{
	Use: "list",
	Short: "List information from scan",
	Run: func(cmd *cobra.Command, args []string) {
		probeResults, err := sqlite.GetProbeResults(dbpath)
		if err != nil {
			logrus.Errorf("could not get probe results: %v\n", err)
		}
		for _, r := range probeResults {
			fmt.Printf("%s:%d (%s)\n", r.Host, r.Port, r.Protocol)
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}