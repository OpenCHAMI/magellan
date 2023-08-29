package cmd

import (
	magellan "davidallendj/magellan/internal"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)


var listCmd = &cobra.Command{
	Use: "list",
	Short: "List information from scan",
	Run: func(cmd *cobra.Command, args []string) {
		probeResults, err := magellan.GetStates(dbpath)
		if err != nil {
			logrus.Errorf("could not get probe results: %v\n", err)
		}
		fmt.Printf("%v\n", probeResults)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}