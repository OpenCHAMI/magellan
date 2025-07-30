package cmd

import (
	"encoding/json"
	"fmt"
	"sync"

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
  // list supported reset types
  magellan power x1000c0s0b3n0 -l
  // more realistic usage
  magellan power -u USER -p PASS -f collect.yaml x1000c0s0b3n0 x1000c0s0b3n1 x1000c0s0b3n2
  // inventory from stdin
  magellan collect -v ... | magellan power -f - x1000c0s0b3n0`,
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
			nodemap[nodes[i].ClusterID] = nodes[i]
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
				ClusterID: node.ClusterID,
				NodeID:    node.Node_ID,
				ConnConfig: crawler.CrawlerConfig{
					URI:             "https://" + node.Bmc_IP,
					CredentialStore: store,
					Insecure:        insecure,
				},
			})
		}

		// Create the appropriate "action function" based on CLI flags (or lack thereof)
		var action_func func(power.CrawlableNode) string
		if list_reset_types {
			action_func = func(target power.CrawlableNode) string {
				types, err := power.GetResetTypes(target)
				if err != nil {
					log.Error().Err(err).Msgf("failed to get reset types for node %s", target.ClusterID)
					return ""
				}
				return fmt.Sprintf("%s", types)
			}
		} else if reset_type != "" {
			action_func = func(target power.CrawlableNode) string {
				// TODO: Some kind of validation might be nice here, but ResetType
				// is a custom string type, so a direct typecast works fine for now.
				err := power.ResetComputerSystem(target, redfish.ResetType(reset_type))
				if err != nil {
					log.Error().Err(err).Msgf("failed to reset node %s", target.ClusterID)
					return "failure"
				}
				return "success"
			}
		} else {
			action_func = func(target power.CrawlableNode) string {
				state, err := power.GetPowerState(target)
				if err != nil {
					log.Error().Err(err).Msgf("failed to get power state of node %s", target.ClusterID)
					state = "unknown"
				}
				return string(state)
			}
		}

		// Actual node operations, in parallel
		results := concurrent_helper(concurrency, target_nodes, action_func)
		power.LogoutBMCSessions()
		for node, status := range results {
			fmt.Printf("%s:\t%s\n", node, status)
		}
	},
}

func concurrent_helper(concurrency int, targets []power.CrawlableNode, runner func(power.CrawlableNode) string) map[string]string {
	type NodeInfo struct {
		ClusterID string
		Result    string
	}
	dataChannel := make(chan power.CrawlableNode, 1)
	returnChannel := make(chan NodeInfo, concurrency)
	results := make(map[string]string, len(targets))
	var wg sync.WaitGroup

	// Worker threads
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				// Get next work item, if any
				target, ok := <-dataChannel
				if !ok {
					wg.Done()
					return
				}
				// Perform work and return result
				returnChannel <- NodeInfo{target.ClusterID, runner(target)}
			}
		}()
	}
	// Receive worker results
	go func() {
		for {
			info, ok := <-returnChannel
			if !ok {
				break
			}
			results[info.ClusterID] = info.Result
		}
		wg.Done()
	}()

	// Dispatch data and wait for processing completion
	for i := range targets {
		dataChannel <- targets[i]
	}
	close(dataChannel)
	wg.Wait()
	// Ensure the receiver thread has also finished
	wg.Add(1)
	close(returnChannel)
	wg.Wait()

	return results
}

func init() {
	// Alternative actions from the default power-state query
	PowerCmd.Flags().BoolVarP(&list_reset_types, "list-reset-types", "l", false, "List supported Redfish reset types")
	PowerCmd.Flags().StringVarP(&reset_type, "reset-type", "r", "", "Redfish reset type to perform")
	PowerCmd.MarkFlagsMutuallyExclusive("reset-type", "list-reset-types")

	// Normal config options
	PowerCmd.Flags().StringP("inventory-file", "f", "", "YAML file containing node inventory")
	PowerCmd.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username")
	PowerCmd.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password")
	PowerCmd.Flags().String("secrets-file", "", "Set path to the node secrets file")
	PowerCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL errors")
	PowerCmd.Flags().String("cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	PowerCmd.Flags().StringP("format", "F", FORMAT_JSON, "Set the output format (json|yaml)")

	// Bind flags to config properties
	checkBindFlagError(viper.BindPFlag("power.cacert", PowerCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlags(PowerCmd.Flags()))

	rootCmd.AddCommand(PowerCmd)
}
