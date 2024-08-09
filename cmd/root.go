// The cmd package implements the interface for the magellan CLI. The files
// contained in this package only contains implementations for handling CLI
// arguments and passing them to functions within magellan's internal API.
//
// Each CLI subcommand will have at least one corresponding internal file
// with an API routine that implements the command's functionality. The main
// API routine will usually be the first function defined in the fill.
//
// For example:
//
//	cmd/scan.go    --> internal/scan.go ( magellan.ScanForAssets() )
//	cmd/collect.go --> internal/collect.go ( magellan.CollectAll() )
//	cmd/list.go    --> none (doesn't have API call since it's simple)
//	cmd/update.go  --> internal/update.go ( magellan.UpdateFirmware() )
package cmd

import (
	"fmt"
	"net"
	"os"
	"os/user"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/pkg/client"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	currentUser *user.User
	accessToken string
	format      string
	timeout     int
	concurrency int
	ports       []int
	hosts       []string
	protocol    string
	cacertPath  string
	username    string
	password    string
	cachePath   string
	outputPath  string
	configPath  string
	verbose     bool
	debug       bool
)

// The `root` command doesn't do anything on it's own except display
// a help message and then exits.
var rootCmd = &cobra.Command{
	Use:   "magellan",
	Short: "Redfish-based BMC discovery tool",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

// This Execute() function is called from main to run the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	currentUser, _ = user.Current()
	cobra.OnInitialize(InitializeConfig)
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", -1, "set the number of concurrent processes")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 5, "set the timeout")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "set the config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "set to enable/disable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "set to enable/disable debug messages")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "set the access token")
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", currentUser.Username), "set the scanning result cache path")

	// bind viper config flags with cobra
	viper.BindPFlag("concurrency", rootCmd.Flags().Lookup("concurrency"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("verbose", rootCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("cache", rootCmd.Flags().Lookup("cache"))
	viper.BindPFlags(rootCmd.Flags())
}

// InitializeConfig() initializes a new config object by loading it
// from a file given a non-empty string.
//
// See the 'LoadConfig' function in 'internal/config' for details.
func InitializeConfig() {
	if configPath != "" {
		err := magellan.LoadConfig(configPath)
		if err != nil {
			log.Error().Err(err).Msg("failed to load config")
		}
	}
}

// SetDefaults() resets all of the viper properties back to their
// default values.
//
// TODO: This function should probably be moved to 'internal/config.go'
// instead of in this file.
func SetDefaults() {
	currentUser, _ = user.Current()
	viper.SetDefault("threads", 1)
	viper.SetDefault("timeout", 5)
	viper.SetDefault("config", "")
	viper.SetDefault("verbose", false)
	viper.SetDefault("debug", false)
	viper.SetDefault("cache", fmt.Sprintf("/tmp/%s/magellan/magellan.db", currentUser.Username))
	viper.SetDefault("scan.hosts", []string{})
	viper.SetDefault("scan.ports", []int{})
	viper.SetDefault("scan.subnets", []string{})
	viper.SetDefault("scan.subnet-masks", []net.IP{})
	viper.SetDefault("scan.disable-probing", false)
	viper.SetDefault("collect.driver", []string{"redfish"})
	viper.SetDefault("collect.host", client.Host)
	viper.SetDefault("collect.user", "")
	viper.SetDefault("collect.pass", "")
	viper.SetDefault("collect.protocol", "tcp")
	viper.SetDefault("collect.output", "/tmp/magellan/data/")
	viper.SetDefault("collect.force-update", false)
	viper.SetDefault("collect.ca-cert", "")
	viper.SetDefault("bmc-host", "")
	viper.SetDefault("bmc-port", 443)
	viper.SetDefault("user", "")
	viper.SetDefault("pass", "")
	viper.SetDefault("transfer-protocol", "HTTP")
	viper.SetDefault("protocol", "tcp")
	viper.SetDefault("firmware-url", "")
	viper.SetDefault("firmware-version", "")
	viper.SetDefault("component", "")
	viper.SetDefault("secure-tls", false)
	viper.SetDefault("status", false)

}
