package cmd

import (
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
// TODO: set ports to scan automatically with set driver

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
	rootCmd.PersistentFlags().IntVar(&threads, "threads", -1, "set the number of threads")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 10, "set the timeout")
	
	rootCmd.PersistentFlags().StringVar(&dbpath, "db.path", "/tmp/magellan.db", "set the probe storage path")
	
}
