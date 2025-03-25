package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
)

var (
	showCache bool
)

// The `list` command provides an easy way to show what was found
// and stored in a cache database from a scan. The data that's stored
// is what is consumed by the `collect` command with the --cache flag.
var ListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.ExactArgs(0),
	Short: "List information stored in cache from a scan",
	Long: "Prints all of the host and associated data found from performing a scan.\n" +
		"See the 'scan' command on how to perform a scan.\n\n" +
		"Examples:\n" +
		"  magellan list\n" +
		"  magellan list --cache ./assets.db",
	Run: func(cmd *cobra.Command, args []string) {
		// check if we just want to show cache-related info and exit
		if showCache {
			fmt.Printf("cache: %s\n", cachePath)
			return
		}

		// load the assets found from scan
		scannedResults, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			log.Error().Err(err).Msg("failed to get scanned assets")
		}
		format = strings.ToLower(format)
		if format == "json" {
			b, err := json.Marshal(scannedResults)
			if err != nil {
				log.Error().Err(err).Msgf("failed to unmarshal scanned results")
			}
			fmt.Printf("%s\n", string(b))
		} else {
			for _, r := range scannedResults {
				fmt.Printf("%s:%d (%s) @%s\n", r.Host, r.Port, r.Protocol, r.Timestamp.Format(time.UnixDate))
			}
		}
	},
}

func init() {
	ListCmd.Flags().StringVar(&format, "format", "", "Set the output format (json|default)")
	ListCmd.Flags().BoolVar(&showCache, "cache-info", false, "Show cache information and exit")
	rootCmd.AddCommand(ListCmd)
}
