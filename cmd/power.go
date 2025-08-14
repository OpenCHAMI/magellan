package cmd

import (
	"fmt"
	"sync"

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/power"
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
var powerCmd = &cobra.Command{
	Use: "power <node-id>...",
	Example: `  // get power state
  magellan power x1000c0s0b3n0
  // perform a particular type of reset
  magellan power x1000c0s0b3n0 -r On
  magellan power x1000c0s0b3n0 -r PowerCycle
  // list supported reset types
  magellan power x1000c0s0b3n0 -l
  // more realistic usage
  magellan power -u USER -p PASS -f collect.json x1000c0s0b3n0 x1000c0s0b3n1 x1000c0s0b3n2
  // inventory from stdin
  magellan collect -v ... | magellan power -f - x1000c0s0b3n0`,
	Short: "Get and set node power states",
	Long:  "Determine and control the power states of nodes found by a previous inventory crawl.\nSee the 'scan' and 'crawl' commands for further details.",
	Run: func(cmd *cobra.Command, args []string) {
		// Read node inventory from CLI flag, or default `collect` YAML output
		var datafile string
		if viper.IsSet("power.inventory-file") {
			datafile = viper.GetString("power.inventory-file")
		} else {
			datafile = viper.GetString("collect.output-file")
			log.Info().Msgf("parsing default inventory file from 'collect': %s", datafile)
		}
		// Parse node inventory
		nodes, err := power.ParseInventory(datafile, viper.GetString("power.format"))
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to parse inventory file %s", datafile)
			// log.Fatal().Msg() does os.Exit(1) for us
		}

		// Set the minimum/maximum number of concurrent processes
		concurrency := viper.GetInt("concurrency")
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(args), 1, 10000)
		}

		// Build secret store, using Viper parameters
		store := util.BuildSecretStore()

		// Index nodes by xname, for faster lookup...
		nodemap := make(map[string]bmc.Node, len(nodes))
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
				NodeID:    node.NodeID,
				ConnConfig: crawler.CrawlerConfig{
					URI:             "https://" + node.BmcIP,
					CredentialStore: store,
					Insecure:        viper.GetBool("insecure"),
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
	powerCmd.Flags().BoolVarP(&list_reset_types, "list-reset-types", "l", false, "List supported Redfish reset types")
	powerCmd.Flags().StringVarP(&reset_type, "reset-type", "r", "", "Redfish reset type to perform")
	powerCmd.MarkFlagsMutuallyExclusive("reset-type", "list-reset-types")

	// Normal config options
	powerCmd.Flags().StringP("inventory-file", "f", "", "YAML file containing node inventory")
	powerCmd.Flags().StringP("username", "u", "", "Set the master BMC username")
	powerCmd.Flags().StringP("password", "p", "", "Set the master BMC password")
	powerCmd.Flags().String("secrets-file", "", "Set path to the node secrets file")
	powerCmd.Flags().BoolP("insecure", "i", false, "Ignore SSL errors")
	powerCmd.Flags().String("cacert", "", "Set the path to CA cert file (defaults to system CAs when blank)")
	powerCmd.Flags().StringP("format", "F", util.FORMAT_JSON, "Set the output format (json|yaml)")

	// Bind flags to config properties
	checkBindFlagError(viper.BindPFlag("power.inventory-file", powerCmd.Flags().Lookup("inventory-file")))
	checkBindFlagError(viper.BindPFlag("username", powerCmd.Flags().Lookup("username")))
	checkBindFlagError(viper.BindPFlag("password", powerCmd.Flags().Lookup("password")))
	checkBindFlagError(viper.BindPFlag("cacert", powerCmd.Flags().Lookup("cacert")))
	checkBindFlagError(viper.BindPFlag("insecure", powerCmd.Flags().Lookup("insecure")))
	checkBindFlagError(viper.BindPFlag("secrets.file", powerCmd.Flags().Lookup("secrets-file")))
	checkBindFlagError(viper.BindPFlag("power.format", powerCmd.Flags().Lookup("format")))

	rootCmd.AddCommand(powerCmd)
}
