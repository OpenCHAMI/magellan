package cmd

import (
	magellan "davidallendj/magellan/internal"
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
	Short: "Scan for BMCs",
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
		magellan.StoreStates(dbpath, &probeStates)
	},
}

func init() {
	scanCmd.Flags().Uint8Var(&begin, "begin", 0, "set the starting point for range of IP addresses")
	scanCmd.Flags().Uint8Var(&end, "end", 255, "set the ending point for range of IP addresses")
	scanCmd.Flags().StringSliceVar(&subnets, "subnet", []string{"127.0.0.0"}, "set additional subnets")

	rootCmd.AddCommand(scanCmd)
}
