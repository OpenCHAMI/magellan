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
	Long: `	Collect hardware inventory by interrogating BMC nodes using scan data. This command send request(s) to a collection of hosts running Redfish services found stored from the 'scan' in cache, provided through stdin, or provided using the '-d/--data' flag. 
	
	See the 'scan' command on how to perform a scan to create. 
	
	Input is taken from, in order of precedence: an explicitly set '--cache' file, positional arguments, and piped standard input. An explicit '--cache' is honored even when stdin is a pipe, and stdin is never read from a terminal, so this command behaves the same with and without a TTY. When none of these provide data, the cache path is used as a fallback.
	
	The path to BMC ID mappings can be specified using the '--bmc-id-mappings' flag. This will convert any hosts found  
	
	See 'magellan-collect(1)' for more details. See 'magellan(1)' for a list of available environment variables.
	`,
	Run: func(cmd *cobra.Command, args []string) {
		// get probe states stored in db from scan
		var (
			scannedResults []magellan.RemoteAsset

			// used for processing stdin and --data arguments
			inputData []map[string]any
			temp      = processDataArgs(collectDataArgs, collectInputFormat)
			err       error
		)

		// resolve which assets to collect from: an explicit --cache wins
		// unconditionally, even over a piped stdin, so `scan --cache ... ;
		// collect --cache ...` behaves the same with and without a TTY
		// (see issue #189)
		var stdinInput []map[string]any
		scannedResults, stdinInput, err = loadCollectInput(cachePath, isCacheExplicit(cmd), args, collectInputFormat)
		if err != nil {
			log.Error().Err(err).Msg("failed to load input for collect")
			os.Exit(1)
		}
		temp = append(temp, stdinInput...)

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
	CollectCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	CollectCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	CollectCmd.Flags().StringVar(&secretsFile, "secrets-file", "", "Set path to the node secrets file")
	CollectCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the protocol used to query")
	CollectCmd.Flags().StringVarP(&outputPath, "output-file", "o", "", "Set the path to store collection data in a single file")
	CollectCmd.Flags().StringVarP(&outputDir, "output-dir", "O", "", "Set the path to store collection data using HIVE partitioning")
	CollectCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe")
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

	rootCmd.AddCommand(CollectCmd)
}

// IsStdinEmpty reports whether standard input is known to carry no data,
// without consuming anything from it.
//
// It reports true for character devices (a terminal, /dev/null) and for
// empty regular files (e.g. `< empty.json`). It reports false for pipes,
// sockets and non-empty files. An idle pipe cannot be distinguished from
// one with data in flight without reading from it, so this must not be
// used on its own to decide whether the user piped data in: callers
// choosing between stdin and --cache should prefer an explicit --cache
// instead (see loadCollectInput and issue #189).
func IsStdinEmpty() (bool, error) {
	file, err := os.Stdin.Stat()
	if err != nil {
		return true, fmt.Errorf("failed to stat stdin")
	}

	// terminals and other character devices (e.g. /dev/null) carry no piped data
	if file.Mode()&os.ModeCharDevice != 0 {
		return true, nil
	}

	// a regular file only has data to read when it is non-empty
	if file.Mode().IsRegular() {
		return file.Size() == 0, nil
	}

	// pipes, sockets and friends: data may be present, we cannot tell without reading
	return false, nil
}

// isCacheExplicit reports whether the user asked for a specific cache by
// passing --cache on the command line or by overriding it through the
// environment or a config file (any resolved value that differs from the
// built-in default). The default path stays implicit so that
// `scan | collect` pipelines, which never ask for a cache, keep reading
// standard input instead.
func isCacheExplicit(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("cache")
	if f == nil {
		return false
	}
	return f.Changed || cachePath != f.DefValue
}

// loadCollectInput resolves the scanned assets that 'collect' will
// interrogate, along with any input read from stdin destined for the same
// merge path as the '-d/--data' flag.
//
// Input sources are consulted in this order:
//
//  1. --cache <path>, when set explicitly (flag, environment or config).
//     An explicit cache always wins, even when stdin is a pipe, so
//     `scan --cache ... ; collect --cache ...` behaves identically with
//     and without a TTY. Failing to read a cache the user explicitly asked
//     for is an error; an explicitly empty --cache ("") disables it.
//  2. positional arguments and piped/redirected stdin. Stdin is never read
//     from a terminal, which would block until end of input.
//  3. the cache path as a fallback when neither of the above produced any
//     input. This preserves the historical behavior of empty stdin meaning
//     "read the cache", now applied consistently regardless of TTY. A cache
//     that cannot be read here is only a warning; the caller reports the
//     missing input.
//
// Data passed with '-d/--data' is processed by the caller and merged with
// these results no matter which source was used.
func loadCollectInput(cachePath string, cacheExplicit bool, args []string, inputFormat format.DataFormat) ([]magellan.RemoteAsset, []map[string]any, error) {
	isStdinEmpty, err := IsStdinEmpty()
	if err != nil {
		log.Warn().Err(err).Msg("failed to determine if stdin is empty")
	}
	piped := !isStdinEmpty

	log.Debug().
		Str("cache", cachePath).
		Bool("cache_explicit", cacheExplicit).
		Bool("stdin_piped", piped).
		Send()

	// (1) an explicit --cache is honored unconditionally
	if cacheExplicit && cachePath != "" {
		assets, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read scan results from --cache %q: %w", cachePath, err)
		}
		return assets, nil, nil
	}

	var (
		assets     []magellan.RemoteAsset
		stdinInput []map[string]any
	)

	// (2) positional arguments...
	for _, arg := range args {
		var asset magellan.RemoteAsset
		if err := format.UnmarshalData([]byte(arg), &asset, inputFormat); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal data from standard input")
			continue
		}
		assets = append(assets, asset)
	}

	// ...and piped stdin, but never a terminal (it would block)
	if piped {
		stdinInput = handleArgs(args, inputFormat)
	}

	// (3) fall back to the cache when nothing else produced input; an
	// explicitly empty --cache ("") means "no cache at all"
	cacheDisabled := cacheExplicit && cachePath == ""
	if len(assets) == 0 && len(stdinInput) == 0 && !cacheDisabled {
		cached, err := sqlite.GetScannedAssets(cachePath)
		switch {
		case err == nil:
			assets = cached
		case cachePath != "":
			log.Warn().Err(err).Msgf("failed to get scanned results from cache")
		default:
			log.Warn().Msg("expected '--cache' to be set when stdin is empty")
		}
	}

	return assets, stdinInput, nil
}
