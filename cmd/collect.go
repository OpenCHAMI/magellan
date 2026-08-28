package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cznic/mathutil"
	"github.com/openchami/magellan/internal/cache/sqlite"
	"github.com/openchami/magellan/internal/format"
	magellan "github.com/openchami/magellan/pkg"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
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
	Example: `  # basic collect after scan without making a follow-up request
  magellan collect --cache ./assets.db --cacert ochami.pem -o nodes.yaml -t 30

  # set username and password for all nodes and produce the collected
  # data in a file called 'nodes.yaml'
  magellan collect -u $bmc_username -p $bmc_password -o nodes.yaml

  # run a collect using secrets from the secrets manager
  export MASTER_KEY=$(magellan secrets generatekey)
  magellan secrets store $node_creds_json -f nodes.json
  magellan collect -o nodes.yaml

  # Take the output of 'scan' and input directly into 'collect'
  magellan scan --subnet 172.18.0.0/24 --port 5000 -l info -i -F json | magellan collect -f json --show-output -i

  # Similar to above, but with intermediate step to allow editting 'scan' output using YAML
  magellan scan --subnet 172.18.0.0/24 --port 5000 -l info -i -F yaml > asset.yaml
  magellan collect -d@asset.yaml -f yaml --show-output -i

  # Take the output of 'collect' and input directly into 'send'
  magellan collect -F json -i --show-output | magellan send https://demo.openchami.cluster:8443/hsm/v2
  
  # Complete flow combined as a single line (data format must match all commands)
  magellan scan --subnet 172.18.0.0/24 --port 5000 -l info -i -F json | magellan collect -f json -F json --show-output -i | magellan send -f json https://demo.openchami.cluster:8443/hsm/v2

  # Run 'collect' using environment variables
  SHOW_OUTPUT=true LOG_LEVEL=debug magellan collect -i -d@assets.json
  `,
	Short: "Collect hardware inventory by interrogating BMC nodes using scan data",
	Long: `Collect hardware inventory by interrogating BMC nodes using scan 
data. This command send request(s) to a collection of hosts running Redfish 
services found stored from the 'scan' in cache, provided through stdin, or 
provided using the '-d/--data' flag. 

See 'magellan scan --help' on how to perform a scan to create. 
See 'magellan send --help' on how to send the inventory to a remote host.

The path to BMC ID mappings can be specified using the '--bmc-id-mappings' flag. 
This will convert any hosts found in the mappings file to the each value 
specified.

See 'magellan-collect(1)' for more details. See 'magellan(1)' for a list of 
available environment variables.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		var (
			scannedResults []magellan.RemoteAsset
			isStdinEmpty   bool

			// used for processing stdin and --data arguments
			inputData []map[string]any
			temp      = processDataArgs(collectDataArgs, collectInputFormat)
			err       error
		)

		// use --cache path if stdin is empty
		isStdinEmpty, err = IsStdinEmpty()
		if err != nil {
			log.Warn().Err(err).Msg("failed to determine if stdin is empty")
		}
		log.Debug().
			Str("cache", cachePath).
			Bool("is_stdin_empty", isStdinEmpty).
			Send()
		if isStdinEmpty {
			if cachePath == "" {
				log.Warn().Msg("expected '--cache' to be set when stdin is empty")
			}
			scannedResults, err = sqlite.GetScannedAssets(cachePath)
			if err != nil {
				log.Warn().Err(err).Msgf("failed to get scanned results from cache")
			}
		} else {
			// unmarshal directly from standard input
			for _, arg := range args {
				var asset magellan.RemoteAsset
				err = format.UnmarshalData([]byte(arg), &asset, collectInputFormat)
				if err != nil {
					log.Warn().Err(err).Msg("failed to unmarshal data from standard input")
					continue
				}
				scannedResults = append(scannedResults, asset)
			}

			// otherwise, add the arg to be processed further down
			temp = append(temp, handleArgs(args, collectInputFormat)...)
		}

		// process input provided from stdin and --data flag
		for _, data := range temp {
			if data != nil {
				inputData = append(inputData, data)
			}
		}

		// show the data count that was just loaded as input
		log.Debug().Int("input_count", len(inputData)).Send()

		// build and append target hosts from input data
		for _, dataObject := range inputData {
			// assert that we have certain values in data object
			var (
				asset    magellan.RemoteAsset
				inputRaw []byte
			)
			inputRaw, err = format.MarshalData(dataObject, collectInputFormat)
			if err != nil {
				log.Error().Err(err).Msg("failed to marshal input data")
			}
			err = format.UnmarshalData(inputRaw, &asset, collectInputFormat)
			if err != nil {
				log.Error().Err(err).Msg("failed to unmarshal input data")
			}
			scannedResults = append(scannedResults, asset)
		}

		// check that we have something to actually scan
		if len(scannedResults) == 0 {
			log.Error().Msg("data required to perform collect either from standard input, '--data' flag, or '--cache'")
			os.Exit(1)
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
	CollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username.")
	CollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password.")
	CollectCmd.Flags().StringVar(&secretsFile, "secrets-file", "secrets.json", "Set the secrets file with BMC credentials.")
	CollectCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query.")
	CollectCmd.Flags().StringVarP(&outputPath, "output-file", "o", "", "Set the path to store collection data in a single file.")
	CollectCmd.Flags().StringVarP(&outputDir, "output-dir", "O", "", "Set the path to store collection data using HIVE partitioning.")
	CollectCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe.")
	CollectCmd.Flags().BoolVar(&showOutput, "show-output", false, "Show the output of a collect run.")
	CollectCmd.Flags().VarP(&collectInputFormat, "input-format", "f", "Set the default input data format (json|yaml).")
	CollectCmd.Flags().VarP(&collectOutputFormat, "output-format", "F", "Set the default output data format (json|yaml; can be overridden by file extensions).")
	CollectCmd.Flags().StringVarP(&idMap, "bmc-id-map", "m", "", "Set the BMC ID mapping from raw json data or use @<path> to specify a file path (json or yaml input).")
	CollectCmd.Flags().StringArrayVarP(&collectDataArgs, "data", "d", []string{}, "Set the data as input for collect (prepend @ for files).")

	// set mutually exclusive flags
	CollectCmd.MarkFlagsMutuallyExclusive("output-file", "output-dir")

	// register completion flag functions
	checkRegisterFlagCompletionError(CollectCmd.RegisterFlagCompletionFunc("input-format", completionFormatData))
	checkRegisterFlagCompletionError(CollectCmd.RegisterFlagCompletionFunc("output-format", completionFormatData))

	rootCmd.AddCommand(CollectCmd)
}

func IsStdinEmpty() (bool, error) {
	var (
		file         os.FileInfo
		fromTerminal bool
		err          error
	)
	file, err = os.Stdin.Stat()
	if err != nil {
		return true, fmt.Errorf("failed to stat stdin")
	}

	// check if there's data from terminal or piped in
	fromTerminal = (file.Mode() & os.ModeCharDevice) == 0

	return !fromTerminal, nil
}
