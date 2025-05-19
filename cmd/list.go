package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var (
	showCache        bool
	listOutputFormat string
)

// The `list` command provides an easy way to show what was found
// and stored in a cache database from a scan. The data that's stored
// is what is consumed by the `collect` command with the --cache flag.
var ListCmd = &cobra.Command{
	Use: "list",
	Example: `  magellan list
  magellan list --cache ./assets.db
  magellan list --cache-info
	`,
	Args:  cobra.ExactArgs(0),
	Short: "List information stored in cache from a scan",
	Long: "Prints all of the host and associated data found from performing a scan.\n" +
		"See the 'scan' command on how to perform a scan.",
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
		switch strings.ToLower(listOutputFormat) {
		case FORMAT_JSON:
			b, err := json.Marshal(scannedResults)
			if err != nil {
				log.Error().Err(err).Msgf("failed to unmarshal cached data to JSON")
			}
			fmt.Printf("%s\n", string(b))
		case FORMAT_YAML:
			b, err := yaml.Marshal(scannedResults)
			if err != nil {
				log.Error().Err(err).Msgf("failed to unmarshal cached data to YAML")
			}
			fmt.Printf("%s\n", string(b))
		case FORMAT_LIST:
			for _, r := range scannedResults {
				fmt.Printf("%s:%d (%s) @%s\n", r.Host, r.Port, r.Protocol, r.Timestamp.Format(time.UnixDate))
			}
		default:
			log.Error().Msg("unrecognized format")
			os.Exit(1)
		}
	},
}

func init() {
	ListCmd.Flags().StringVarP(&listOutputFormat, "format", "F", FORMAT_LIST, "Set the output format (json|yaml|table)")
	ListCmd.Flags().BoolVar(&showCache, "cache-info", false, "Show cache information and exit")
	rootCmd.AddCommand(ListCmd)
}
