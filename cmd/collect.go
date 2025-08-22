package cmd

import (
	"fmt"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/internal/util"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The `collect` command fetches data from a collection of BMC nodes.
// This command should be ran after the `scan` to find available hosts
// on a subnet.
var collectCmd = &cobra.Command{
	Use: "collect",
	Example: `  // basic collect after scan without making a follow-up request
  magellan collect --cache ./assets.db --cacert ochami.pem -o nodes.yaml -t 30

  // set username and password for all nodes and produce the collected
  // data in a file called 'nodes.yaml'
  magellan collect -u $bmc_username -p $bmc_password -o nodes.yaml

  // run a collect using secrets from the secrets manager
  export MASTER_KEY=$(magellan secrets generatekey)
  magellan secrets store $node_creds_json -f nodes.json
  magellan collect -o nodes.yaml`,
	Short: "Collect system information by interrogating BMC node",
	Long:  "Send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache.\nSee the 'scan' command on how to perform a scan.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate the specified file format
		collectOutputFormat := viper.GetString("collect.format")
		if collectOutputFormat != util.FORMAT_JSON && collectOutputFormat != util.FORMAT_YAML {
			return fmt.Errorf("specified format '%s' is invalid, must be (json|yaml)", collectOutputFormat)
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		scannedResults, err := sqlite.GetScannedAssets(viper.GetString("cache"))
		if err != nil {
			log.Error().Err(err).Msgf("failed to get scanned results from cache")
		}

		// set the minimum/maximum number of concurrent processes
		concurrency := viper.GetInt("concurrency")
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(scannedResults), 1, 10000)
		}

		// Build secret store, using Viper parameters
		store := util.BuildSecretStore()

		// set the collect parameters from CLI params
		params := &magellan.CollectParams{
			Timeout:     viper.GetInt("timeout"),
			Concurrency: concurrency,
			Verbose:     viper.GetBool("verbose"),
			CaCertPath:  viper.GetString("cacert"),
			OutputPath:  viper.GetString("collect.output-file"),
			OutputDir:   viper.GetString("collect.output-dir"),
			Format:      viper.GetString("collect.format"),
			ForceUpdate: viper.GetBool("collect.force-update"),
			AccessToken: viper.GetString("access-token"),
			SecretStore: store,
			BMCIDMap:    viper.GetString("collect.bmc-id-map"),
		}

		// show all of the 'collect' parameters being set from CLI if verbose
		if viper.GetBool("verbose") {
			log.Debug().Any("params", params)
		}

		_, err = magellan.CollectInventory(&scannedResults, params)
		if err != nil {
			log.Error().Err(err).Msg("failed to collect data")
		}
	},
}

func init() {
	addFlag("username", collectCmd, "username", "u", "", "Set the master BMC username")
	addFlag("password", collectCmd, "password", "p", "", "Set the master BMC password")
	addFlag("cacert", collectCmd, "cacert", "", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	addFlag("secrets.file", collectCmd, "secrets-file", "", "", "Set path to the node secrets file")
	addFlag("collect.protocol", collectCmd, "protocol", "", "tcp", "Set the protocol used to query")
	addFlag("collect.output-file", collectCmd, "output-file", "o", "", "Set the path to store collection data using HIVE partitioning")
	addFlag("collect.output-dir", collectCmd, "output-dir", "O", "", "Set the path to store collection data using HIVE partitioning")
	addFlag("collect.force-update", collectCmd, "force-update", "", false, "Set flag to force update data sent to SMD")
	addFlag("collect.format", collectCmd, "format", "F", util.FORMAT_JSON, "Set the output format (json|yaml)")
	addFlag("collect.bmc-id-map", collectCmd, "bmc-id-map", "m", "", "Set the BMC ID mapping from raw json data or use @<path> to specify a file path (json or yaml input)")

	rootCmd.AddCommand(collectCmd)
}
