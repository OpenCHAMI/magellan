package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCHAMI/magellan/internal/db/sqlite"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List information from scan",
	Run: func(cmd *cobra.Command, args []string) {
		probeResults, err := sqlite.GetProbeResults(cachePath)
		if err != nil {
			logrus.Errorf("failed toget probe results: %v\n", err)
		}
		format = strings.ToLower(format)
		if format == "json" {
			b, _ := json.Marshal(probeResults)
			fmt.Printf("%s\n", string(b))
		} else {
			for _, r := range probeResults {
				fmt.Printf("%s:%d (%s) @ %s\n", r.Host, r.Port, r.Protocol, r.Timestamp.Format(time.UnixDate))
			}
		}
	},
}

func init() {
	listCmd.Flags().StringVar(&format, "format", "", "set the output format")
	rootCmd.AddCommand(listCmd)
}
