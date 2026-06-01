package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/internal/format"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

var (
	collectInputFormat  format.DataFormat = format.FORMAT_JSON
	collectOutputFormat format.DataFormat = format.FORMAT_JSON
	collectDataArgs     []string
)

// The `collect` command fetches data from a collection of BMC nodes.
// This command should be ran after the `scan` to find available hosts
// on a subnet.
var CollectCmd = &cobra.Command{
	Use: "collect",
	Example: `  // basic collect after scan without making a follow-up request
  magellan collect --cache ./assets.db --cacert ochami.pem -o nodes.yaml -t 30

  // set username and password for all nodes and produce the collected
  // data in a file called 'nodes.yaml'
  magellan collect -u $bmc_username -p $bmc_password -o nodes.yaml

  // run a collect using secrets from the secrets manager
  export MASTER_KEY=$(magellan secrets generatekey)
  magellan secrets store $node_creds_json -f nodes.json
  magellan collect -o nodes.yaml

  // Take the output of 'scan' and input directly into 'collect'
  magellan scan --subnet 172.18.0.0/24 --port 5000 -l info -i -F json | ./magellan collect -f json --show-output -i
  
  // Complete flow combined as a single line
  magellan scan --subnet 172.18.0.0/24 --port 5000 -l info -i -F json | ./magellan collect -f json --show-output -i | magellan send https://smd.example.com
  `,
	Short: "Collect system information by interrogating BMC node",
	Long:  "Send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache.\nSee the 'scan' command on how to perform a scan.",
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		var (
			scannedResults []magellan.RemoteAsset
			err            error
		)
		if cachePath != "" {
			scannedResults, err = sqlite.GetScannedAssets(cachePath)
			if err != nil {
				log.Error().Err(err).Msgf("failed to get scanned results from cache")
			}
		} else {
			// try to get the data from standard input or the -d/--data flag
			for _, arg := range args {
				var asset magellan.RemoteAsset
				err = format.UnmarshalData([]byte(arg), &asset, collectInputFormat)
				if err != nil {
					log.Warn().Err(err).Msg("failed to unmarshal data from standard input")
					continue
				}
			}

			var inputData []map[string]any
			temp := append(handleArgs(args), processDataArgs(sendDataArgs)...)
			for _, data := range temp {
				if data != nil {
					inputData = append(inputData, data)
				}
			}
			if len(inputData) == 0 {
				log.Error().Msg("data required with standard input or -d/--data flag")
				os.Exit(1)
			}

			// show the data that was just loaded as input
			// inputRaw, _ := json.MarshalIndent(inputData, "", "  ")
			log.Debug().Int("endpoint_count", len(inputData)).Send()

			// build and append target hosts from input data
			// for _, dataObject := range inputData {
			// 	// assert that we have certain values in data object
			// 	var asset magellan.RemoteAsset
			// 	format.UnmarshalData()
			// 	host := dataObject["host"].(string)

			// }
		}

		// set the minimum/maximum number of concurrent processes
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(scannedResults), 1, 10000)
		}

		// use secret store for BMC credentials, and/or credential CLI flags
		var store secrets.SecretStore
		if username != "" && password != "" {
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Msgf("one or both of --username and --password NOT passed, attempting to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Err(err).Msg("failed to open local secrets store")
			}

			// Temporarily override username/password of each BMC if one of those
			// flags is passed. The expectation is that if the flag is specified
			// on the command line, it should be used.
			if username != "" {
				log.Info().Msg("--username passed, temporarily overriding all usernames from secret store with value")
			}
			if password != "" {
				log.Info().Msg("--password passed, temporarily overriding all passwords from secret store with value")
			}
			switch s := store.(type) {
			case *secrets.StaticStore:
				if username != "" {
					s.Username = username
				}
				if password != "" {
					s.Password = password
				}
			case *secrets.LocalSecretStore:
				for k := range s.Secrets {
					if creds, err := bmc.GetBMCCredentials(store, k); err != nil {
						log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials")
					} else {
						if username != "" {
							creds.Username = username
						}
						if password != "" {
							creds.Password = password
						}

						if newCreds, err := json.Marshal(creds); err != nil {
							log.Error().Str("id", k).Err(err).Msg("failed to override BMC credentials: marshal error")
						} else {
							err = s.StoreSecretByID(k, string(newCreds))
							if err != nil {
								log.Error().Err(err).Str("id", k).Msg("failed to store secret by ID")
							}
						}
					}
				}
			}
		}

		// set the collect parameters from CLI params
		params := &magellan.CollectParams{
			Timeout:      timeout,
			Concurrency:  concurrency,
			OutputPath:   outputPath,
			OutputDir:    outputDir,
			Insecure:     insecure,
			OutputFormat: collectOutputFormat,
			InputFormat:  collectInputFormat,
			SecretStore:  store,
			BMCIDMap:     idMap,
		}

		// show all of the 'collect' parameters being set from CLI if verbose
		log.Debug().Any("params", params).Send()

		inventory, err := magellan.CollectInventory(&scannedResults, params)
		if err != nil {
			log.Error().Err(err).Msg("failed to collect data")
		}

		if showOutput {
			output, err := format.MarshalData(inventory, collectOutputFormat)
			if err != nil {
				log.Error().Msgf("failed to marshal inventory to %s", strings.ToUpper(collectOutputFormat.String()))
				os.Exit(1)
			}
			fmt.Println(string(output))
		}
	},
}

func init() {
	CollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	CollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	CollectCmd.Flags().StringVar(&secretsFile, "secrets-file", "", "Set path to the node secrets file")
	CollectCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query")
	CollectCmd.Flags().StringVarP(&outputPath, "output-file", "o", "", "Set the path to store collection data in a single file")
	CollectCmd.Flags().StringVarP(&outputDir, "output-dir", "O", "", "Set the path to store collection data using HIVE partitioning")
	CollectCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe")
	CollectCmd.Flags().BoolVar(&showOutput, "show", false, "Show the output of a collect run")
	CollectCmd.Flags().BoolVar(&showOutput, "show-output", false, "Show the output of a collect run")
	CollectCmd.Flags().VarP(&collectInputFormat, "input-format", "f", "Set the default input data format (json|yaml)")
	CollectCmd.Flags().VarP(&collectOutputFormat, "output-format", "F", "Set the default output data format (json|yaml; can be overridden by file extensions)")
	CollectCmd.Flags().StringVarP(&idMap, "bmc-id-map", "m", "", "Set the BMC ID mapping from raw json data or use @<path> to specify a file path (json or yaml input)")
	CollectCmd.Flags().StringArrayVarP(&collectDataArgs, "data", "d", []string{}, "Set the data as input for collect (prepend @ for files)")

	// set mutually exclusive flags
	CollectCmd.MarkFlagsMutuallyExclusive("output-file", "output-dir")

	// register completion flag functions
	checkRegisterFlagCompletionError(CollectCmd.RegisterFlagCompletionFunc("input-format", completionFormatData))
	checkRegisterFlagCompletionError(CollectCmd.RegisterFlagCompletionFunc("output-format", completionFormatData))

	// bind flags to config properties
	checkBindFlagError(viper.BindPFlag("collect.protocol", CollectCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("collect.output-file", CollectCmd.Flags().Lookup("output-file")))
	checkBindFlagError(viper.BindPFlag("collect.output-dir", CollectCmd.Flags().Lookup("output-dir")))
	// checkBindFlagError(viper.BindPFlag("collect.force-update", CollectCmd.Flags().Lookup("force-update")))
	// checkBindFlagError(viper.BindPFlag("collect.cacert", CollectCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(CollectCmd.Flags()))

	rootCmd.AddCommand(CollectCmd)
}
