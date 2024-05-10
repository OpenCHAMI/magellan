package cmd

import (
	"fmt"
	"net"
	"os"
	"path"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/db/sqlite"

	"github.com/cznic/mathutil"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	begin          uint8
	end            uint8
	subnets        []string
	subnetMasks    []net.IP
	disableProbing bool
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for BMC nodes on a network",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("subnets in cmd: %v\n", subnets)
		// set hosts to use for scanning
		hostsToScan := []string{}
		if len(hosts) > 0 {
			hostsToScan = hosts
		} else {
			for i, subnet := range subnets {
				if len(subnet) <= 0 {
					return
				}

				if len(subnetMasks) < i+1 {
					subnetMasks = append(subnetMasks, net.IP{255, 255, 255, 0})
				}

				hostsToScan = append(hostsToScan, magellan.GenerateHosts(subnet, &subnetMasks[i])...)
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
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(hostsToScan), 1, 255)
		}
		probeStates := magellan.ScanForAssets(hostsToScan, portsToScan, concurrency, timeout, disableProbing, verbose)
		if verbose {
			for _, r := range probeStates {
				fmt.Printf("%s:%d (%s)\n", r.Host, r.Port, r.Protocol)
			}
		}

		// make the dbpath dir if needed
		err := os.MkdirAll(path.Dir(dbpath), 0766)
		if err != nil {
			fmt.Printf("failed tomake database directory: %v", err)
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
	scanCmd.Flags().IPSliceVar(&subnetMasks, "subnet-mask", []net.IP{}, "set the subnet masks to use for network")
	scanCmd.Flags().BoolVar(&disableProbing, "disable-probing", false, "disable probing scanned results for BMC nodes")

	viper.BindPFlag("scan.hosts", scanCmd.Flags().Lookup("host"))
	viper.BindPFlag("scan.ports", scanCmd.Flags().Lookup("port"))
	viper.BindPFlag("scan.subnets", scanCmd.Flags().Lookup("subnet"))
	viper.BindPFlag("scan.subnet-masks", scanCmd.Flags().Lookup("subnet-mask"))
	viper.BindPFlag("scan.disable-probing", scanCmd.Flags().Lookup("disable-probing"))

	rootCmd.AddCommand(scanCmd)
}
