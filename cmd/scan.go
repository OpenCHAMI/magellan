package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/db/sqlite"

	"github.com/cznic/mathutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	subnets        []string
	subnetMasks    []net.IP
	disableProbing bool
)

// The `scan` command is usually the first step to using the CLI tool.
// This command will perform a network scan over a subnet by supplying
// a list of subnets, subnet masks, and additional IP address to probe.
//
// See the `ScanForAssets()` function in 'internal/scan.go' for details
// related to the implementation.
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for BMC nodes on a network",
	Long: "Perform a net scan by attempting to connect to each host and port specified and getting a response. " +
		"If the '--disable-probe` flag is used, the tool will not send another request to probe for available " +
		"Redfish services.\n\n" +
		"Example:\n" +
		"  magellan scan --subnet 172.16.0.0/24 --host 10.0.0.101\n" +
		"  magellan scan --subnet 172.16.0.0 --subnet-mask 255.255.255.0 --cache ./assets.db",
	Run: func(cmd *cobra.Command, args []string) {
		var (
			hostsToScan []string
			portsToScan []int
		)

		// start by adding `--host` supplied to scan
		if len(hosts) > 0 {
			hostsToScan = hosts
		}

		// add hosts from `--subnets` and `--subnet-mask`
		for i, subnet := range subnets {
			// subnet string is empty so nothing to do here
			if subnet == "" {
				continue
			}

			// NOTE: should we check if subnet is valid here or is it done elsewhere (maybe in GenerateHosts)?

			// no subnet masks supplied so add a default one for class C private networks
			if len(subnetMasks) < i+1 {
				subnetMasks = append(subnetMasks, net.IP{255, 255, 255, 0})
			}

			// generate a slice of all hosts to scan from subnets
			hostsToScan = append(hostsToScan, magellan.GenerateHosts(subnet, &subnetMasks[i])...)
		}

		// add ports to use for scanning
		if len(ports) > 0 {
			portsToScan = ports
		} else {
			// no ports supplied so only use defaults
			portsToScan = magellan.GetDefaultPorts()
		}

		// scan and store scanned data in cache
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(hostsToScan), 1, 255)
		}
		probeStates := magellan.ScanForAssets(hostsToScan, portsToScan, concurrency, timeout, disableProbing, verbose)
		if verbose {
			format = strings.ToLower(format)
			if format == "json" {
				b, _ := json.Marshal(probeStates)
				fmt.Printf("%s\n", string(b))
			} else {
				for _, r := range probeStates {
					fmt.Printf("%s:%d (%s) @ %s\n", r.Host, r.Port, r.Protocol, r.Timestamp.Format(time.UnixDate))
				}
			}
		}

		// make the dbpath dir if needed
		err := os.MkdirAll(path.Dir(cachePath), 0766)
		if err != nil {
			fmt.Printf("failed tomake database directory: %v", err)
		}

		sqlite.InsertProbeResults(cachePath, &probeStates)
	},
}

func init() {
	scanCmd.Flags().StringSliceVar(&hosts, "host", []string{}, "set additional hosts to scan")
	scanCmd.Flags().IntSliceVar(&ports, "port", []int{}, "set the ports to scan")
	scanCmd.Flags().StringVar(&format, "format", "", "set the output format")
	scanCmd.Flags().StringSliceVar(&subnets, "subnet", []string{}, "set additional subnets")
	scanCmd.Flags().IPSliceVar(&subnetMasks, "subnet-mask", []net.IP{}, "set the subnet masks to use for network (must match number of subnets)")
	scanCmd.Flags().BoolVar(&disableProbing, "disable-probing", false, "disable probing scanned results for BMC nodes")

	viper.BindPFlag("scan.hosts", scanCmd.Flags().Lookup("host"))
	viper.BindPFlag("scan.ports", scanCmd.Flags().Lookup("port"))
	viper.BindPFlag("scan.subnets", scanCmd.Flags().Lookup("subnet"))
	viper.BindPFlag("scan.subnet-masks", scanCmd.Flags().Lookup("subnet-mask"))
	viper.BindPFlag("scan.disable-probing", scanCmd.Flags().Lookup("disable-probing"))

	rootCmd.AddCommand(scanCmd)
}
