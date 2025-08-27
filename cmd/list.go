package cmd

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/internal/format"
	"github.com/rs/zerolog/log"

	"github.com/spf13/cobra"
)

var (
	showCache        bool
	listOutputFormat format.DataFormat = format.FORMAT_LIST
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

		output, err := format.MarshalData(scannedResults, listOutputFormat)
		if err != nil {

		}
		log.Printf(string(output))
	},
}

func init() {
	ListCmd.Flags().VarP(&listOutputFormat, "format", "F", "Set the output format (list|json|yaml)")
	ListCmd.Flags().BoolVar(&showCache, "cache-info", false, "Show cache information and exit")
	rootCmd.AddCommand(ListCmd)
}
