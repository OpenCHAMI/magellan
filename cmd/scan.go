package cmd

import (
	magellan "davidallendj/magellan/internal"
	"davidallendj/magellan/internal/db/sqlite"
	"fmt"

	"github.com/cznic/mathutil"
	"github.com/spf13/cobra"
)

var (
	begin   uint8
	end     uint8
	subnets []string
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
			for _, subnet := range subnets {
				hostsToScan = append(hostsToScan, magellan.GenerateHosts(subnet, begin, end)...)
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
		probeStates := magellan.ScanForAssets(hostsToScan, portsToScan, threads, timeout)
		fmt.Printf("probe states: %v\n", probeStates)
		sqlite.InsertProbeResults(dbpath, &probeStates)
	},
}

func init() {
	scanCmd.PersistentFlags().StringSliceVar(&hosts, "host", []string{}, "set additional hosts to scan")
	scanCmd.PersistentFlags().IntSliceVar(&ports, "port", []int{}, "set the ports to scan")
	scanCmd.Flags().Uint8Var(&begin, "begin", 0, "set the starting point for range of IP addresses")
	scanCmd.Flags().Uint8Var(&end, "end", 255, "set the ending point for range of IP addresses")
	scanCmd.Flags().StringSliceVar(&subnets, "subnet", []string{}, "set additional subnets")

	rootCmd.AddCommand(scanCmd)
}
