package cmd

import (
	"context"
	"fmt"
	"net"

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
		if viper.GetBool("daemon.print-only") {
			do_output = daemon.OutputToStdout
		} else {
			do_output = daemon.OutputToSMD
			// TODO: Connect to SMD
		}

		// Load the assets found from scan
		scannedResults, err := sqlite.GetScannedAssets(cachePath)
		if err != nil {
			log.Error().Err(err).Msg("failed to get scanned assets")
			return
		}

		// Networking prep
		// Determine what port our server is running on
		_, port, err := net.SplitHostPort(viper.GetString("daemon.server-addr"))
		if err != nil {
			return
		}
		// We may need to do IP matching later, so collect a list of
		// all our local network links once, ahead of time
		var localAddrs []net.Addr
		if !viper.IsSet("daemon.callback-addr") {
			localAddrs, err = net.InterfaceAddrs()
			if err != nil {
				log.Error().Err(err).Msg("failed to get local network addresses")
				return
			}
		}
		fmt.Println("GlobalUnicast LinkLocalUnicast Private Loopback")
		for i := range localAddrs {
			a := localAddrs[i]
			switch t := a.(type) {
			case *net.IPAddr:
				// Single addresses aren't interesting, probably
				continue
			case *net.IPNet:
				// Networks may be interesting; BMCs could live there
				fmt.Printf("%v\t%v\t%v\t%v\tIPNet: %s : %s (%s)\n",
					t.IP.IsGlobalUnicast(),
					t.IP.IsLinkLocalUnicast(),
					t.IP.IsPrivate(),
					t.IP.IsLoopback(),
					t, t.IP.String(), t.Mask,
				)
				// TODO: Select "interesting" networks for later consideration, based on the above fields
			}
		}

		// Start callback server (sends updates to SMD)
		serverCtx, serverCancel := context.WithCancel(context.Background())
		serverDone := make(chan error, 1)
		go daemon.RunServer(serverCtx, serverDone, viper.GetString("daemon.server-addr"))
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

			// Determine callback address from BMC to this daemon's server
			var callbackAddr string
			if viper.IsSet("daemon.callback-addr") {
				// Callback address provided by user; use it
				// NOTE: This is needed when  our local IP and port
				// aren't visible to the BMCs for direct
				// connections, e.g. when we're behind NAT or a
				// port forwarding configuration
				callbackAddr = viper.GetString("daemon.callback-addr")
			} else {
				// Search our local IP addresses for the best
				// prefix match with our target BMC, and assume
				// that's the link where the BMC can reach us
				ip := "PLACEHOLDER_IP" // FIXME:
				// Server address is just a port specification; use it to generate callback address
				callbackAddr = fmt.Sprintf("https://%s%s/", ip, port)
			}

			subUri, err := daemon.CreateBMCPowerSubscription(
				crawler.CrawlerConfig{
					URI:             r.Host,
					CredentialStore: store,
					Insecure:        viper.GetBool("daemon.insecure"),
					UseDefault:      true,
				},
				daemon.Subscription{
					// FIXME:
					Destination:      callbackAddr,
					RegistryPrefixes: []string{"registry_prefix"},
					ResourceTypes:    []string{},
					HttpHeaders:      map[string]string{},
					Context:          "",
				},
			)
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
	DaemonCmd.Flags().StringVarP(&secretsFile, "secrets-file", "f", "secrets.json", "Set path to the node secrets file")
	DaemonCmd.Flags().BoolP("insecure", "i", false, "Ignore SSL errors")
	DaemonCmd.Flags().String("server-addr", ":27781", "Where this daemon's server should listen for BMC event callbacks")
	DaemonCmd.Flags().String("callback-addr", "", "Address which BMCs should use to reach this daemon's server. Set this if daemon is behind NAT/port remapping")
	DaemonCmd.Flags().Bool("print-only", false, "Just print BMC status updates, instead of sending them to SMD")

	checkBindFlagError(viper.BindPFlag("daemon.insecure", DaemonCmd.Flags().Lookup("insecure")))
	checkBindFlagError(viper.BindPFlag("daemon.server-addr", DaemonCmd.Flags().Lookup("server-addr")))
	checkBindFlagError(viper.BindPFlag("daemon.callback-addr", DaemonCmd.Flags().Lookup("callback-addr")))
	checkBindFlagError(viper.BindPFlag("daemon.print-only", DaemonCmd.Flags().Lookup("print-only")))

	rootCmd.AddCommand(DaemonCmd)
}
