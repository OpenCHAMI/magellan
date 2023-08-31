package cmd

import (
	"davidallendj/magellan/api/smd"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	timeout       int
	threads       int
	ports         []int
	hosts         []string
	withSecureTLS bool
	certPoolFile  string
	user          string
	pass          string
	dbpath        string
	drivers       []string
	preferredDriver string
	ipmitoolPath  string
)

// TODO: discover bmc's on network (dora)
// TODO: query bmc component information and store in db (?)
// TODO: send bmc component information to smd

var rootCmd = &cobra.Command{
	Use:   "magellan",
	Short: "Tool for BMC discovery",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&user, "user", "", "set the BMC user")
	rootCmd.PersistentFlags().StringVar(&pass, "pass", "", "set the BMC pass")
	rootCmd.PersistentFlags().StringSliceVar(&hosts, "host", []string{}, "set additional hosts")
	rootCmd.PersistentFlags().StringVar(&smd.Host, "smd-host", "localhost", "set the host to the hms-smd API")
	rootCmd.PersistentFlags().IntVar(&threads, "threads", -1, "set the number of threads")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 10, "set the timeout")
	rootCmd.PersistentFlags().IntSliceVar(&ports, "port", []int{}, "set the ports to scan")
	rootCmd.PersistentFlags().StringSliceVar(&drivers, "driver", []string{"redfish"}, "set the driver(s) and fallback drivers to use")
	rootCmd.PersistentFlags().StringVar(&preferredDriver, "preferred-driver", "ipmi", "set the preferred driver to use")
	rootCmd.PersistentFlags().StringVar(&dbpath, "dbpath", ":memory:", "set the probe storage path")
	rootCmd.PersistentFlags().StringVar(&ipmitoolPath, "ipmitool", "/usr/bin/ipmitool", "set the path for ipmitool")
	rootCmd.PersistentFlags().BoolVar(&withSecureTLS, "secure-tls", false, "enable secure TLS")
	rootCmd.PersistentFlags().StringVar(&certPoolFile, "cert-pool", "", "path to CA cert. (defaults to system CAs; used with --secure-tls=true)")
}
