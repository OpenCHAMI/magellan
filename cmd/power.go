package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/power"
	"github.com/OpenCHAMI/magellan/pkg/secrets"
	"github.com/cznic/mathutil"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stmcginnis/gofish/redfish"
)

var (
	list_reset_types bool
	reset_type       string
)

// The `power` command gets and sets power states for a collection of BMC nodes.
// This command should be run after `collect`, as it requires an existing node inventory.
var PowerCmd = &cobra.Command{
	Use: "power <node-id>...",
	Example: `  // get power state
  magellan power x1000c0s0b3n0
  // perform a particular type of reset
  magellan power x1000c0s0b3n0 -r On
  magellan power x1000c0s0b3n0 -r PowerCycle

  // These power states are actually Redfish ResetTypes. They may vary by BMC, but should include:
  //  - On
  //  - PowerCycle
  //  - Graceful[Shutdown|Restart]
  //  - Force[Off|On|Restart]
  //  - PushPowerButton`,
	Short: "Get and set node power states",
	Long:  "Determine and control the power states of nodes found by a previous inventory crawl.\nSee the 'scan' and 'crawl' commands for further details.",
	Run: func(cmd *cobra.Command, args []string) {
		// Read node inventory from CLI flag, or default `collect` YAML output
		var datafile string
		if viper.IsSet("inventory-file") {
			datafile = viper.GetString("inventory-file")
		} else {
			datafile = viper.GetString("collect.output-file")
			log.Info().Msgf("parsing default inventory file from 'collect': %s", datafile)
		}
		// Parse node inventory
		nodes, err := power.ParseInventory(datafile)
		if err != nil {
			log.Error().Err(err).Msgf("failed to parse inventory file %s", datafile)
			return
		}

		// Set the minimum/maximum number of concurrent processes
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(args), 1, 10000)
		}

		// Use secret store for BMC credentials, and/or credential CLI flags
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
							s.StoreSecretByID(k, string(newCreds))
						}
					}
				}
			}
		}

		// Index nodes by xname, for faster lookup...
		nodemap := make(map[string]power.NodeViaBMC, len(nodes))
		for i := range nodes {
			nodemap[nodes[i].Xname] = nodes[i]
		}
		// ...and select the ones requested by the user
		target_nodes := make([]power.CrawlableNode, 0, len(args))
		for i := range args {
			node, found := nodemap[args[i]]
			if !found {
				log.Error().Msgf("target node '%s' not found in inventory; skipping", args[i])
				continue
			}
			target_nodes = append(target_nodes, power.CrawlableNode{
				Xname:    node.Xname,
				BmcIndex: node.Bmc_Index,
				ConnConfig: crawler.CrawlerConfig{
					URI:             fmt.Sprintf("%s://%s", viper.GetString("power.scheme"), node.Bmc_IP),
					CredentialStore: store,
					Insecure:        insecure,
				},
			})
		}

		// Actual node operations
		if reset_type != "" {
			for _, target := range target_nodes {
				// TODO: Some kind of validation might be nice here, but ResetType
				// is a custom string type, so a direct typecast works fine for now.
				power.ResetComputerSystem(target, redfish.ResetType(reset_type))
			}
		} else {
			for _, target := range target_nodes {
				power.GetPowerState(target)
			}
		}
		power.LogoutBMCSessions()
	},
}

func init() {
	// Alternative actions from the default power-state query
	PowerCmd.Flags().StringVarP(&reset_type, "reset-type", "r", "", "Redfish reset type to perform")

	// Normal config options
	PowerCmd.Flags().StringP("inventory-file", "i", "", "YAML file containing node inventory")
	PowerCmd.Flags().StringP("username", "u", "", "Set the master BMC username")
	PowerCmd.Flags().StringP("password", "p", "", "Set the master BMC password")
	PowerCmd.Flags().String("secrets-file", "", "Set path to the node secrets file")
	PowerCmd.Flags().String("scheme", "https", "Set the scheme (\"http\" or \"https\") used to contact BMCs")
	PowerCmd.Flags().String("cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	PowerCmd.Flags().StringP("format", "F", FORMAT_JSON, "Set the output format (json|yaml)")

	// Bind flags to config properties
	checkBindFlagError(viper.BindPFlag("power.scheme", PowerCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("power.cacert", PowerCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(PowerCmd.Flags()))

	rootCmd.AddCommand(PowerCmd)
}
