package cmd

import (
	"context"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/daemon"
	"github.com/OpenCHAMI/magellan/pkg/secrets"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
  magellan daemon -i -u username -p password
  magellan daemon --print-only`,
	Args:  cobra.ExactArgs(0),
	Short: "Launch in daemon mode, listening for user commands and BMC events",
	Long: "Creates Redfish event subscriptions for power events (falling back to polling if necessary) and sends updates to SMD.\n" +
		"Also listens for user commands, such as instructing a node to change its power state.\n" +
		"Monitors nodes found via earlier scans; see the 'scan' command for information about performing a scan.",
	Run: func(cmd *cobra.Command, args []string) {
		// Set up crawler config for BMC connections
		var (
			store      secrets.SecretStore
			fetchCreds bool
			err        error
		)

		if username != "" && password != "" {
			fetchCreds = false
			// First, try and load credentials from --username and --password if both are set.
			log.Debug().Msgf("--username and --password specified, using them for BMC credentials")
			store = secrets.NewStaticStore(username, password)
		} else {
			fetchCreds = true
			// Alternatively, locate specific credentials (falling back to default) and override those
			// with --username or --password if either are passed.
			log.Debug().Msgf("one or both of --username and --password NOT passed, will attempt to obtain missing credentials from secret store at %s", secretsFile)
			if store, err = secrets.OpenStore(secretsFile); err != nil {
				log.Error().Err(err).Msg("failed to open local secrets store")
			}
		}

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

		// Start callback server (sends updates to SMD)
		serverCtx, serverCancel := context.WithCancel(context.Background())
		serverDone := make(chan error, 1)
		go daemon.RunServer(serverCtx, serverDone, ":1337") // FIXME: Port number
		// This should be started before we create our Redfish
		// subscriptions, in case the BMCs do an immediate check to
		// ensure the server exists, or try to send the last few
		// minutes' worth of cached logs.

		// Subscribe to Redfish power events, or add to polling list if sub fails
		var subUris, pollUris []string
		for _, r := range scannedResults {
			store := store
			if fetchCreds {
				// Either none of the flags were passed or only one of them were; get
				// credentials from secrets store to fill in the gaps.
				bmcCreds, _ := bmc.GetBMCCredentials(store, r.Host)
				nodeCreds := secrets.StaticStore{
					Username: bmcCreds.Username,
					Password: bmcCreds.Password,
				}

				// If either of the flags were passed, override the fetched
				// credentials with them.
				if username != "" {
					log.Info().Msg("--username was set, overriding username for this BMC")
					nodeCreds.Username = username
				}
				if password != "" {
					log.Info().Msg("--password was set, overriding password for this BMC")
					nodeCreds.Password = password
				}

				store = &nodeCreds
			}

			var config = crawler.CrawlerConfig{
				URI:             r.Host,
				CredentialStore: store,
				Insecure:        insecure,
				UseDefault:      true,
			}

			subUri, err := daemon.CreateBMCPowerSubscription(config, daemon.Subscription{
				// FIXME:
				Destination:      "https://callback.server/endpoint",
				RegistryPrefixes: []string{"registry_prefix"},
				ResourceTypes:    []string{},
				HttpHeaders:      map[string]string{},
				Context:          "",
			})
			if err == nil {
				subUris = append(subUris, subUri)
			} else {
				log.Error().Err(err).Msgf("could not create event subscription on %s, falling back to polling", r.Host)
				var pollUri string // TODO:
				pollUris = append(pollUris, pollUri)
			}
			do_output(r.Host, redfish.PowerSubsystem{}) // FIXME:
		}

		// TODO: Start polling routine; wait for termination

		// Shut down callback server
		serverCancel()
		log.Info().Err(<-serverDone).Msg("callback server exited")

		// TODO: Clean up subscriptions
	},
}

func init() {
	DaemonCmd.Flags().StringVarP(&username, "username", "u", "", "Set the username for the BMC")
	DaemonCmd.Flags().StringVarP(&password, "password", "p", "", "Set the password for the BMC")
	DaemonCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL errors")
	DaemonCmd.Flags().StringVarP(&secretsFile, "secrets-file", "f", "secrets.json", "Set path to the node secrets file")
	DaemonCmd.Flags().BoolVar(&printOnly, "print-only", false, "Just print node status updates, instead of sending them to SMD")

	checkBindFlagError(viper.BindPFlag("crawl.insecure", DaemonCmd.Flags().Lookup("insecure")))

	rootCmd.AddCommand(DaemonCmd)
}
