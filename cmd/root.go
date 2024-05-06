package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	accessToken     string
	timeout         int
	threads         int
	ports           []int
	hosts           []string
	protocol        string
	cacertPath      string
	user            string
	pass            string
	dbpath          string
	drivers         []string
	preferredDriver string
	ipmitoolPath    string
	outputPath      string
	verbose         bool
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

func LoadAccessToken() (string, error) {
	// try to load token from env var
	testToken := os.Getenv("OCHAMI_ACCESS_TOKEN")
	if testToken != "" {
		return testToken, nil
	}

	// try reading access token from a file
	b, err := os.ReadFile(tokenPath)
	if err == nil {
		return string(b), nil
	}

	// TODO: try to load token from config
	return "", fmt.Errorf("could not load from environment variable or file")
}

func init() {
	rootCmd.PersistentFlags().IntVar(&threads, "threads", -1, "set the number of threads")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "set the timeout")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", true, "set verbose flag")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "set the access token")
	rootCmd.PersistentFlags().StringVar(&dbpath, "db.path", "/tmp/magellan/magellan.db", "set the probe storage path")
}
