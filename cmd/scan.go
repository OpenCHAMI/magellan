package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"

	"github.com/OpenCHAMI/magellan/internal/cache/sqlite"
	magellan "github.com/OpenCHAMI/magellan/pkg"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	urlx "github.com/OpenCHAMI/magellan/internal/url"
	"github.com/cznic/mathutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	scheme         string
	subnets        []string
	subnetMask     net.IPMask
	targetHosts    [][]string
	disableProbing bool
	disableCache   bool
	format         string
	include        []string
)

// The `scan` command is usually the first step to using the CLI tool.
// This command will perform a network scan over a subnet by supplying
// a list of subnets, subnet masks, and additional IP address to probe.
//
// See the `ScanForAssets()` function in 'internal/scan.go' for details
// related to the implementation.
var ScanCmd = &cobra.Command{
	Use: "scan urls...",
	Example: `
  // assumes host https://10.0.0.101:443
  magellan scan 10.0.0.101

  // assumes subnet using HTTPS and port 443 except for specified host
  magellan scan http://10.0.0.101:80 https://user:password@10.0.0.102:443 http://172.16.0.105:8080 --subnet 172.16.0.0/24

  // assumes hosts http://10.0.0.101:8080 and http://10.0.0.102:8080
  magellan scan 10.0.0.101 10.0.0.102 https://172.16.0.10:443 --port 8080 --protocol tcp

  // assumes subnet using default unspecified subnet-masks
  magellan scan --subnet 10.0.0.0

  // assumes subnet using HTTPS and port 443 with specified CIDR
  magellan scan --subnet 10.0.0.0/16

  // assumes subnet using HTTP and port 5000 similar to 192.168.0.0/16
  magellan scan --subnet 192.168.0.0 --protocol tcp --scheme https --port 5000 --subnet-mask 255.255.0.0

  // assumes subnet without CIDR has a subnet-mask of 255.255.0.0
  magellan scan --subnet 10.0.0.0/24 --subnet 172.16.0.0 --subnet-mask 255.255.0.0 --cache ./assets.db`,
	Short: "Scan to discover BMC nodes on a network",
	Long: "Perform a net scan by attempting to connect to each host and port specified and getting a response.\n" +
		"Each host is passed *with a full URL* including the protocol and port. Additional subnets can be added\n" +
		"by using the '--subnet' flag and providing an IP address on the subnet as well as a CIDR. If no CIDR is\n" +
		"provided, then the subnet mask specified with the '--subnet-mask' flag will be used instead (will use\n" +
		"default mask if not set).\n\n" +
		"Similarly, any host provided with no port will use either the ports specified\n" +
		"with `--port` or the default port used with each specified protocol. The default protocol is 'tcp' unless\n" +
		"specified. The `--scheme` flag works similarly and the default value is 'https' in the host URL or with the\n" +
		"'--protocol' flag.\n\n" +
		"If the '--disable-probe` flag is used, the tool will not send another request to probe for available.\n" +
		"Redfish and JAWS services. This is not recommended, since the extra request makes the scan a bit more reliable\n" +
		"for determining which hosts to collect inventory data.\n\n",
	Run: func(cmd *cobra.Command, args []string) {
		// add default ports for hosts if none are specified with flag
		if len(ports) == 0 {
			if debug {
				log.Debug().Msg("adding default ports")
			}
			ports = magellan.GetDefaultPorts()
		}

		// format and combine flag and positional args
		targetHosts = append(targetHosts, urlx.FormatHosts(args, ports, scheme, verbose)...)

		for _, subnet := range subnets {
			// generate a slice of all hosts to scan from subnets
			subnetHosts := magellan.GenerateHostsWithSubnet(subnet, &subnetMask, ports, scheme)
			targetHosts = append(targetHosts, subnetHosts...)
		}

		// if there are no target hosts, then there's nothing to do
		if len(targetHosts) <= 0 {
			log.Warn().Msg("nothing to do (no valid target hosts)")
			return
		} else {
			if len(targetHosts[0]) <= 0 {
				log.Warn().Msg("nothing to do (no valid target hosts)")
				return
			}
		}

		// show the parameters going into the scan
		if debug {
			combinedTargetHosts := []string{}
			for _, targetHost := range targetHosts {
				combinedTargetHosts = append(combinedTargetHosts, targetHost...)
			}
			c := map[string]any{
				"hosts":           combinedTargetHosts,
				"cache":           cachePath,
				"concurrency":     concurrency,
				"protocol":        protocol,
				"subnets":         subnets,
				"subnet-mask":     subnetMask.String(),
				"cert":            cacertPath,
				"disable-probing": disableProbing,
				"disable-caching": disableCache,
			}
			b, _ := json.MarshalIndent(c, "", "    ")
			fmt.Printf("%s", string(b))
		}

		// set the number of concurrent requests (1 request per BMC node)
		//
		// NOTE: The number of concurrent job is equal to the number of hosts by default.
		// The max concurrent jobs cannot be greater than the number of hosts.
		if concurrency <= 0 {
			concurrency = len(targetHosts)
		} else {
			concurrency = mathutil.Clamp(len(targetHosts), 1, len(targetHosts))
		}

		// scan and store scanned data in cache
		foundAssets := magellan.ScanForAssets(&magellan.ScanParams{
			TargetHosts:    targetHosts,
			Scheme:         scheme,
			Protocol:       protocol,
			Concurrency:    concurrency,
			Timeout:        timeout,
			DisableProbing: disableProbing,
			Verbose:        verbose,
			Debug:          debug,
			Insecure:       insecure,
			Include:        include,
		})

		if len(foundAssets) > 0 && debug {
			log.Info().Any("assets", foundAssets).Msgf("found assets from scan")
		}

		if len(foundAssets) == 0 {
			log.Warn().Msg("Scan complete. No responsive assets were found.")
			return
		}

		switch format {
		case "json", "yaml":
			var output []byte
			var err error

			if format == "json" {
				output, err = json.MarshalIndent(foundAssets, "", "  ")
			} else {
				output, err = yaml.Marshal(foundAssets)
			}

			if err != nil {
				log.Error().Err(err).Msgf("Failed to marshal output to %s", format)
				return
			}

			if outputPath != "" {
				err := os.WriteFile(outputPath, output, 0644)
				if err != nil {
					log.Error().Err(err).Msgf("Failed to write to file: %s", outputPath)
				} else {
					log.Info().Msgf("Scan results successfully written to %s", outputPath)
				}
			} else {
				fmt.Println(string(output))
			}

		case "db":
			if !disableCache && cachePath != "" {
				err := os.MkdirAll(path.Dir(cachePath), 0755)
				if err != nil {
					log.Printf("failed to make cache directory: %v", err)
				}
				err = sqlite.InsertScannedAssets(cachePath, foundAssets...)
				if err != nil {
					log.Error().Err(err).Msg("failed to write scanned assets to cache")
				} else if verbose {
					log.Info().Msgf("Saved assets to cache: %s", cachePath)
				}
			}
		default:
			log.Error().Msgf("unknown format specified: %s. Please use 'db', 'json', or 'yaml'.", format)
		}

	},
}

func init() {
	ScanCmd.Flags().IntSliceVar(&ports, "port", nil, "Adds additional ports to scan for each host with unspecified ports.")
	ScanCmd.Flags().StringVar(&scheme, "scheme", "https", "Set the default scheme to use if not specified in host URI. (default is 'https')")
	ScanCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the default protocol to use in scan. (default is 'tcp')")
	ScanCmd.Flags().StringSliceVar(&subnets, "subnet", nil, "Add additional hosts from specified subnets to scan.")
	ScanCmd.Flags().IPMaskVar(&subnetMask, "subnet-mask", net.IPv4Mask(255, 255, 255, 0), "Set the default subnet mask to use for with all subnets not using CIDR notation.")
	ScanCmd.Flags().BoolVar(&disableProbing, "disable-probing", false, "Disable probing found assets for Redfish service(s) running on BMC nodes")
	ScanCmd.Flags().BoolVar(&disableCache, "disable-cache", false, "Disable saving found assets to a cache database specified with 'cache' flag")
	ScanCmd.Flags().BoolVar(&insecure, "insecure", true, "Skip TLS certificate verification during probe")
	ScanCmd.Flags().StringVarP(&format, "format", "F", "db", "Output format (db, json, yaml)")
	ScanCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file path (for json/yaml formats)")
	ScanCmd.Flags().StringSliceVar(&include, "include", []string{"bmcs"}, "Asset types to scan for (bmcs, pdus)")

	checkBindFlagError(viper.BindPFlag("scan.ports", ScanCmd.Flags().Lookup("port")))
	checkBindFlagError(viper.BindPFlag("scan.scheme", ScanCmd.Flags().Lookup("scheme")))
	checkBindFlagError(viper.BindPFlag("scan.protocol", ScanCmd.Flags().Lookup("protocol")))
	checkBindFlagError(viper.BindPFlag("scan.subnets", ScanCmd.Flags().Lookup("subnet")))
	checkBindFlagError(viper.BindPFlag("scan.subnet-masks", ScanCmd.Flags().Lookup("subnet-mask")))
	checkBindFlagError(viper.BindPFlag("scan.disable-probing", ScanCmd.Flags().Lookup("disable-probing")))
	checkBindFlagError(viper.BindPFlag("scan.disable-cache", ScanCmd.Flags().Lookup("disable-cache")))

	rootCmd.AddCommand(ScanCmd)
}
