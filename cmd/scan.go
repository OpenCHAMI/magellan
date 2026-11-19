package cmd

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/openchami/magellan/internal/cache/sqlite"
	"github.com/openchami/magellan/internal/format"
	magellan "github.com/openchami/magellan/pkg"
	"github.com/rs/zerolog/log"

	"github.com/cznic/mathutil"
	urlx "github.com/openchami/magellan/internal/url"
	"github.com/spf13/cobra"
)

var (
	scheme           string
	subnets          []string
	subnetMask       net.IPMask
	include          []string
	disableProbing   bool
	disableCache     bool
	scanOutputFormat format.DataFormat
)

// The `scan` command is usually the first step to using the CLI tool.
// This command will perform a network scan over a subnet by supplying
// a list of subnets, subnet masks, and additional IP address to probe.
//
// See the `ScanForAssets()` function in 'internal/scan.go' for details
// related to the implementation.
var ScanCmd = &cobra.Command{
	Use: "scan",
	Example: `  # assumes host https://10.0.0.101:443
  magellan scan 10.0.0.101 --insecure

  # assumes subnet using HTTPS and port 443 except for specified host
  magellan scan http://10.0.0.101:80 https://$username:$password@10.0.0.102:443 http://172.16.0.105:8080 --subnet 172.16.0.0/24

  # assumes hosts http://10.0.0.101:8080 and http://10.0.0.102:8080
  magellan scan 10.0.0.101 10.0.0.102 https://172.16.0.10:443 --port 8080 --protocol tcp

  # assumes subnet using default unspecified subnet-masks
  magellan scan --subnet 10.0.0.0 -i

  # assumes subnet using HTTPS and port 443 with specified CIDR
  magellan scan --subnet 10.0.0.0/16 -i

  # same as above example but output is in JSON without caching
  magellan scan --subnet 10.0.0.0/16 -i -F json --disable-cache

  # assumes subnet using HTTP and port 5000 similar to 192.168.0.0/16
  magellan scan --subnet 192.168.0.0 --protocol tcp --scheme https --port 5000 --subnet-mask 255.255.0.0

  # assumes subnet without CIDR has a subnet-mask of 255.255.0.0
  magellan scan --subnet 10.0.0.0 --subnet 172.16.0.0 --subnet-mask 255.255.0.0 --cache ./assets.db

  # pipe 'scan' output as input for 'scan' (requires that 'collect' accept the same data format as 'scan')
  magellan secrets generatekey -f key.txt
  export MASTER_KEY=$(cat key.txt)
  magellan secrets store default $username:$password -f secrets.json
  magellan scan --subnet 172.18.0.0/24 --port 5000 -i -F yaml | magellan collect -i -f yaml --secrets-file secrets.json
  
  // perform scan to find emulator BMCS by setting environment variables
  make emulator
  SCAN_SUBNET=172.18.0.0/24 SCAN_PORTS=5000 magellan scan -l info --insecure`,

	Short: "Perform network scan to discover BMC nodes.",
	Long: `Perform a network scan by attempting to connect to each host and port specified 
and getting a response. Each host is passed *with a full URL* including the 
protocol and port. Additional subnets can be added by using the '--subnet' flag 
and providing an IP address on the subnet as well as a CIDR. If no CIDR is 
provided, then the subnet mask specified with the '--subnet-mask' flag will be 
used instead (will use default mask if not set).

Similarly, any host provided with no port will use either the ports specified 
with '--port' or the default port used with each specified protocol. The default
protocol is 'tcp' unless specified. The '--scheme' flag works similarly and the
default value is 'https' in the host URL or with the '--protocol' flag.

If the '--disable-probing' flag is used, the tool will not send another request to
probe for available. Redfish and JAWS services. This is not recommended, since 
the extra request makes the scan a bit more reliable for determining which hosts
to collect inventory data.

The output of the 'scan' can be piped as input to 'collect', but both commands 
must use the same data format specified with '-f/--input-format' and 
'-F/--output-format'.

See 'magellan-scan(1)' for more details. See 'magellan(1)' for a list of available environment variables.
	`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if scheme != "https" && scheme != "http" {
			return fmt.Errorf("scan scheme must be http or https")
		}
		if protocol != "tcp" && protocol != "tcp4" && protocol != "tcp6" {
			return fmt.Errorf("scan protocol must be tcp, tcp4, or tcp6")
		}
		if timeout <= 0 {
			return fmt.Errorf("scan timeout must be positive")
		}
		if len(include) == 0 {
			return fmt.Errorf("scan requires at least one asset type")
		}
		for _, assetType := range include {
			if assetType != "bmcs" && assetType != "pdus" {
				return fmt.Errorf("scan asset types must be bmcs or pdus")
			}
		}
		if outputPath != "" && scanOutputFormat == "" {
			return fmt.Errorf("scan output file requires an output format")
		}
		// add default ports for hosts if none are specified with flag
		if len(ports) == 0 {
			ports = magellan.GetDefaultPorts()
			log.Debug().Ints("ports", ports).Msg("default ports")
		}

		// format and combine flag and positional args
		for _, port := range ports {
			if port < 1 || port > 65535 {
				return fmt.Errorf("scan port out of range: %d", port)
			}
		}
		for index, host := range args {
			if !strings.Contains(host, "://") {
				host = scheme + "://" + host
			}
			parsed, err := url.ParseRequestURI(host)
			if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return fmt.Errorf("invalid scan target at argument %d (use an HTTP(S) host without credentials or query parameters)", index+1)
			}
			if port := parsed.Port(); port != "" {
				value, err := strconv.Atoi(port)
				if err != nil || value < 1 || value > 65535 {
					return fmt.Errorf("invalid scan target port at argument %d", index+1)
				}
			}
		}
		targetHosts := urlx.FormatHosts(args, ports, scheme)

		for _, subnet := range subnets {
			// generate a slice of all hosts to scan from subnets
			subnetHosts := magellan.GenerateHostsWithSubnet(subnet, &subnetMask, ports, scheme)
			if len(subnetHosts) == 0 {
				return fmt.Errorf("invalid or empty scan subnet")
			}
			targetHosts = append(targetHosts, subnetHosts...)
		}

		// if there are no target hosts, then there's nothing to do
		if len(targetHosts) == 0 {
			return fmt.Errorf("nothing to do (no valid target hosts)")
		}

		// show the parameters going into the scan
		log.Debug().Any("flags", map[string]any{
			"hosts":           "set '--log-level' to 'trace' to show",
			"cache":           cachePath,
			"concurrency":     concurrency,
			"protocol":        protocol,
			"subnets":         subnets,
			"subnet-mask":     subnetMask.String(),
			"cert":            cacertPath,
			"disable-probing": disableProbing,
			"disable-caching": disableCache,
		}).Send()

		// set the number of concurrent requests (1 request per BMC node)
		//
		// NOTE: The number of concurrent job is equal to the number of hosts by default.
		// The max concurrent jobs cannot be greater than the number of hosts.
		if concurrency <= 0 {
			concurrency = len(targetHosts)
		} else {
			concurrency = mathutil.Clamp(concurrency, 1, len(targetHosts))
		}

		// scan and store scanned data in cache
		foundAssets := magellan.ScanForAssets(&magellan.ScanParams{
			TargetHosts:    targetHosts,
			Scheme:         scheme,
			Protocol:       protocol,
			Concurrency:    concurrency,
			Timeout:        timeout,
			DisableProbing: disableProbing,
			Insecure:       insecure,
			Include:        include,
		})

		if len(foundAssets) > 0 {
			log.Trace().Any("assets", foundAssets).Msgf("found assets from scan")
		} else {
			return fmt.Errorf("no responsive assets found; check network reachability, TLS, and probe settings")
		}

		if scanOutputFormat != "" {
			switch scanOutputFormat {
			case format.FORMAT_JSON, format.FORMAT_YAML:
				var (
					output []byte
					err    error
				)

				output, err = format.MarshalData(foundAssets, scanOutputFormat)
				if err != nil {
					return fmt.Errorf("failed to marshal scan output: %w", err)
				}
				if outputPath != "" {
					err := os.WriteFile(outputPath, output, 0644)
					if err != nil {
						return fmt.Errorf("failed to write scan output: %w", err)
					} else {
						log.Debug().Msgf("scan results written to %s", outputPath)
					}
				} else {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), string(output)); err != nil {
						return fmt.Errorf("failed to write scan output: %w", err)
					}
				}

			default:
				return fmt.Errorf("unknown scan output format: %s", scanOutputFormat)
			}
		}

		// write to a cache file if not disabled at specified path
		if !disableCache && cachePath != "" {
			err := os.MkdirAll(path.Dir(cachePath), 0755)
			if err != nil {
				return fmt.Errorf("failed to make cache directory: %w", err)
			}
			err = sqlite.InsertScannedAssets(cachePath, foundAssets...)
			if err != nil {
				return fmt.Errorf("failed to write scanned assets to cache: %w", err)
			}
			log.Debug().Str("path", cachePath).Msg("saved assets to cache")
		}
		return nil
	},
}

func init() {
	ScanCmd.Flags().IntSliceVar(&ports, "port", nil, "Adds additional ports to scan for each host with unspecified ports.")
	ScanCmd.Flags().StringVar(&scheme, "scheme", "https", "Set the default scheme to use if not specified in host URI.")
	ScanCmd.Flags().StringVar(&protocol, "protocol", "tcp", "Set the default protocol to use in scan.")
	ScanCmd.Flags().StringSliceVar(&subnets, "subnet", nil, "Add additional hosts from specified subnets to scan.")
	ScanCmd.Flags().IPMaskVar(&subnetMask, "subnet-mask", net.IPv4Mask(255, 255, 255, 0), "Set the default subnet mask to use for with all subnets not using CIDR notation.")
	ScanCmd.Flags().BoolVar(&disableProbing, "disable-probing", false, "Disable probing found assets for Redfish service(s) running on BMC nodes.")
	ScanCmd.Flags().BoolVar(&disableCache, "disable-cache", false, "Disable saving found assets to a cache database specified with 'cache' flag.")
	ScanCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe.")
	ScanCmd.Flags().VarP(&scanOutputFormat, "output-format", "F", "Sets the output format (json, yaml).")
	ScanCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Sets the output file path (for json/yaml formats).")
	ScanCmd.Flags().StringSliceVar(&include, "include", []string{"bmcs"}, "Asset types to scan for (bmcs, pdus).")

	rootCmd.AddCommand(ScanCmd)
}
