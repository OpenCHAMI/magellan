package cmd

import (
	"fmt"
	"time"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/pkg/daemon"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/stmcginnis/gofish/redfish"
)

var (
	printOnly bool
)

// The `daemon` command launches several services to support continuous node
// status monitoring. It monitors nodes found during previous scans.
// These include:
//   - User command listener
//   - Long-running SMD connection
//   - Redfish event endpoint (receives event callbacks from BMCs)
var DaemonCmd = &cobra.Command{
	Use: "daemon",
	Example: `  magellan daemon
  magellan daemon --print-only`,
	Args:  cobra.ExactArgs(0),
	Short: "Launch in daemon mode, listening for user commands and BMC events",
	Long: "Creates Redfish event subscriptions for power events (falling back to polling if necessary) and sends updates to SMD.\n" +
		"Also listens for user commands, such as instructing a node to change its power state.\n" +
		"Monitors nodes found via earlier scans; see the 'scan' command for information about performing a scan.",
	Run: func(cmd *cobra.Command, args []string) {
		// Set appropriate output function
		var do_output func(string, redfish.PowerSubsystem)
		if printOnly {
			do_output = daemon.OutputToStdout
		} else {
			do_output = daemon.OutputToSMD
			// TODO: Connect to SMD
		}

		// Load the assets found from scan
		scannedResults, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			log.Error().Err(err).Msg("failed to get scanned assets")
		}

		// TODO: Start callback server

		// Subscribe to Redfish power events, or add to polling list if sub fails
		for _, r := range scannedResults {
			fmt.Printf("%s:%d (%s) @%s\n", r.Host, r.Port, r.Protocol, r.Timestamp.Format(time.UnixDate)) // FIXME:

			var config = crawler.CrawlerConfig{
				// TODO: Build a crawler config for the current node
			}

			err = daemon.CreateBMCPowerSubscription(config, daemon.Subscription{
				// FIXME:
				Destination:      "https://callback.server/endpoint",
				RegistryPrefixes: []string{"registry_prefix"},
				ResourceTypes:    []string{},
				HttpHeaders:      map[string]string{},
				Context:          "",
			})
			if err != nil {
				log.Error().Err(err).Msg("could not create event subscription on %s, falling back to polling")
				// TODO:
				continue
			}
			// TODO: Start callback server (sends updates to SMD)
			do_output(r.Host, redfish.PowerSubsystem{})
		}

		// TODO: Start polling routine; wait for termination
	},
}

func init() {
	DaemonCmd.Flags().BoolVar(&printOnly, "print-only", false, "Just print node status updates, instead of sending them to SMD")
	rootCmd.AddCommand(DaemonCmd)
}
