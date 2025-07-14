package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/OpenCHAMI/magellan/pkg/crawler"
	"github.com/OpenCHAMI/magellan/pkg/daemon"
	"github.com/OpenCHAMI/magellan/pkg/secrets"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		var do_output func(daemon.PowerInfo)
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
			log.Error().Err(err).Msg("failed to parse server address")
			return
		}
		// We may need to do IP matching later, so collect a list of
		// all our local network links once, ahead of time
		var ifaceAddrs []net.Addr
		var localNets []*net.IPNet
		if !viper.IsSet("daemon.callback-addr") {
			ifaceAddrs, err = net.InterfaceAddrs()
			if err != nil {
				log.Error().Err(err).Msg("failed to get local network addresses")
				return
			}
			// Filter to only "interesting" networks, on which BMCs could actually live
			// localNets = make([]*net.IPNet, 0, len(ifaceAddrs)) // Do we really need to preallocate this?
			for i := range ifaceAddrs {
				switch t := ifaceAddrs[i].(type) {
				case *net.IPAddr:
					// Single addresses aren't interesting, probably
					continue
				case *net.IPNet:
					// Networks may be interesting; BMCs could live there
					if t.IP.IsLinkLocalUnicast() || (t.IP.IsGlobalUnicast() && t.IP.IsPrivate()) {
						// Capture networks for which our connection is either:
						//  - Link-local (i.e. IPv6 local network)
						//  - Global, but private (i.e. IPv4 local network)
						// Notably, this excludes loopback networks, and interfaces connected to the
						// public internet (not that those should exist on an HPC control node anyway)
						log.Info().Msgf(
							"including local network interface %s for consideration as an event callback target",
							t.String(),
						)
						localNets = append(localNets, t)
					}
				}
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
		var pollUris []string
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
			crawlerConfig := crawler.CrawlerConfig{
				URI:             r.Host,
				CredentialStore: store,
				Insecure:        viper.GetBool("daemon.insecure"),
				UseDefault:      true,
			}

			// Do an immediate poll for initial power state, but don't save the client
			power, err := daemon.PollBMCPowerStates(crawlerConfig, false)
			if err != nil {
				log.Error().Err(err).Msgf("failed to poll %s for power states; BMC will not be monitored", r.Host)
				continue
			}
			for _, p := range power {
				do_output(p)
			}

			// Determine callback address from BMC to this daemon's server
			var callbackAddr string
			if viper.IsSet("daemon.callback-addr") {
				// Callback address provided by user; use it
				// NOTE: This is needed when our local IP and port
				// aren't visible to the BMCs for direct
				// connections, e.g. when we're behind NAT or a
				// port forwarding configuration
				callbackAddr = viper.GetString("daemon.callback-addr")
			} else {
				// Search our local IP addresses for the best
				// prefix match with our target BMC, and assume
				// that's the link where the BMC can reach us
				bmc_ip := net.ParseIP(strings.TrimPrefix(r.Host, "https://"))
				var callback_ip *net.IP = nil
				for _, net := range localNets {
					if net.Contains(bmc_ip) {
						log.Info().Msgf(
							"found local network interface %s over which BMC %s should be able to call us back",
							net.String(), bmc_ip,
						)
						callback_ip = &net.IP
						break
					}
				}
				if callback_ip == nil {
					log.Error().Msgf(
						"could not find a suitable local network interface for BMC %s to call us back over; falling back to polling",
						bmc_ip,
					)
					pollUris = append(pollUris, crawlerConfig.URI)
					continue
				}
				// Generate complete callback address
				callbackAddr = fmt.Sprintf("https://%s%s/", callback_ip.String(), port)
			}

			// Actual subscription creation
			subUri, err := daemon.CreateBMCPowerSubscription(
				crawlerConfig,
				daemon.Subscription{
					// FIXME:
					Destination:      callbackAddr,
					RegistryPrefixes: []string{"registry_prefix"},
					ResourceTypes:    []string{},
					HttpHeaders:      map[string]string{},
					Context:          "",
					Insecure:         viper.GetBool("daemon.insecure"),
				},
			)
			// TODO: It's possible to have both a valid sub URL and
			// an error here, e.g. if sub creation was successful
			// but post-creation updates failed. For now, we assume
			// the subscription is still Mostly Okay™, but there
			// may be a more optimal way to handle this.
			if err != nil {
				if subUri == "" {
					log.Error().Err(err).Msgf("could not create event subscription on %s, falling back to polling", r.Host)
					pollUris = append(pollUris, crawlerConfig.URI)
				} else {
					log.Warn().Err(err).Msgf("partially configured event subscription on %s, continuing with the assumption that it's usable", r.Host)
				}
			}
			// Defer subscription cleanup
			// TODO: Ask a Go wizard about possible performance
			// penalties here — is it better to maintain a list of
			// subscription URLs and manually iterate over those in
			// a single deferred function? This would require
			// reconstructing crawler configs, but that's not too
			// painful, probably.
			// var subUris []string
			// if subUri != "" {
			// 	subUris = append(subUris, subUri)
			// }
			defer daemon.DeleteBMCPowerSubscription(crawlerConfig, subUri)
		}

		// Start polling routine; wait for termination
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, syscall.SIGINT)
		pollTick := time.NewTicker(10 * time.Second)
		do_polling := true
		for do_polling {
			select {
			case <-interrupt:
				do_polling = false
				log.Info().Msg("interrupt received, cleaning up and exiting")
			case <-pollTick.C:
				for _, uri := range pollUris {
					// Do an immediate poll for initial power state, but don't save the client
					power, err := daemon.PollBMCPowerStates(
						crawler.CrawlerConfig{
							URI:             uri,
							CredentialStore: store,
							Insecure:        viper.GetBool("daemon.insecure"),
							UseDefault:      true,
						},
						true,
					)
					if err != nil {
						log.Error().Err(err).Msgf("scheduled poll failed on BMC %s", uri)
						continue
					}
					for _, p := range power {
						do_output(p)
					}
				}
			}
		}
		pollTick.Stop()

		// Shut down callback server and clean up
		serverCancel() // NOTE: Returns even if the server is still closing connections!
		daemon.LogoutPolledBMCs()
		log.Info().Err(<-serverDone).Msg("callback server exited")
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
