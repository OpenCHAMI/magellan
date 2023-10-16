package cmd

import (
	"fmt"
	"os"
	"path"

	magellan "github.com/bikeshack/magellan/internal"
	"github.com/bikeshack/magellan/internal/db/sqlite"

	"github.com/cznic/mathutil"
	"github.com/spf13/cobra"
)

var (
	begin   uint8
	end     uint8
	subnets []string
	subnetMasks []string
	disableProbing bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for BMC nodes on a network",
	Run: func(cmd *cobra.Command, args []string) {
		// set hosts to use for scanning
		hostsToScan := []string{}
		if len(hosts) > 0 {
			hostsToScan = hosts
		} else {
			for i, subnet := range subnets {
				if len(subnetMasks) > 0 {
					hostsToScan = append(hostsToScan, magellan.GenerateHostsWithSubnet(subnet, subnetMasks[i])...)
				} else {
					hostsToScan = append(hostsToScan, magellan.GenerateHosts(subnet)...)
				}
			}
		}

		// set ports to use for scanning
		portsToScan := []int{}
		if len(ports) > 0 {
			portsToScan = ports
		} else {
			portsToScan = append(magellan.GetDefaultPorts(), ports...)
		}

		// scan and store probe data in dbPath
		if threads <= 0 {
			threads = mathutil.Clamp(len(hostsToScan), 1, 255)
		}
		probeStates := magellan.ScanForAssets(hostsToScan, portsToScan, threads, timeout, disableProbing)
		for _, r := range probeStates {
			fmt.Printf("%s:%d (%s)\n", r.Host, r.Port, r.Protocol)
		}

		// make the dbpath dir if needed
		err := os.MkdirAll(path.Dir(dbpath), 0766)
		if err != nil {
			fmt.Printf("could not make database directory: %v", err)
		}

		sqlite.InsertProbeResults(dbpath, &probeStates)
	},
}

func init() {
	scanCmd.Flags().StringSliceVar(&hosts, "host", []string{}, "set additional hosts to scan")
	scanCmd.Flags().IntSliceVar(&ports, "port", []int{}, "set the ports to scan")
	// scanCmd.Flags().Uint8Var(&begin, "begin", 0, "set the starting point for range of IP addresses")
	// scanCmd.Flags().Uint8Var(&end, "end", 255, "set the ending point for range of IP addresses")
	scanCmd.Flags().StringSliceVar(&subnets, "subnet", []string{}, "set additional subnets")
	scanCmd.Flags().StringSliceVar(&subnetMasks, "subnet-mask", []string{}, "set the subnet masks to use for network")
	scanCmd.Flags().BoolVar(&disableProbing, "disable-probing", false, "disable probing scanned results for BMC nodes")

	rootCmd.AddCommand(scanCmd)
}
