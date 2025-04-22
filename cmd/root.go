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
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	FORMAT_JSON = "json"
	FORMAT_YAML = "yaml"
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
	forceUpdate bool
	insecure    bool
	useHive     bool
)

// The `root` command doesn't do anything on it's own except display
// a help message and then exits.
var rootCmd = &cobra.Command{
	Use:   "magellan",
	Short: "Redfish-based BMC discovery tool",
	Long:  "Redfish-based BMC discovery tool with dynamic discovery features.",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			err := cmd.Help()
			if err != nil {
				log.Error().Err(err).Msg("failed to print help")
			}
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
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "j", -1, "Set the number of concurrent processes")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 5, "Set the timeout for requests")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Set the config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Set to enable/disable verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Set to enable/disable debug messages")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "Set the access token")
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", currentUser.Username), "Set the scanning result cache path")

	// bind viper config flags with cobra
	checkBindFlagError(viper.BindPFlag("concurrency", rootCmd.PersistentFlags().Lookup("concurrency")))
	checkBindFlagError(viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout")))
	checkBindFlagError(viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose")))
	checkBindFlagError(viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")))
	checkBindFlagError(viper.BindPFlag("access-token", rootCmd.PersistentFlags().Lookup("verbose")))
	checkBindFlagError(viper.BindPFlag("cache", rootCmd.PersistentFlags().Lookup("cache")))
}

func checkBindFlagError(err error) {
	if err != nil {
		log.Error().Err(err).Msg("failed to bind cobra/viper flag")
	}
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
	viper.SetDefault("cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", currentUser.Username))
	viper.SetDefault("scan.hosts", []string{})
	viper.SetDefault("scan.ports", []int{})
	viper.SetDefault("scan.subnets", []string{})
	viper.SetDefault("scan.subnet-masks", []net.IP{})
	viper.SetDefault("scan.disable-probing", false)
	viper.SetDefault("scan.disable-cache", false)
	viper.SetDefault("collect.host", host)
	viper.SetDefault("collect.username", "")
	viper.SetDefault("collect.password", "")
	viper.SetDefault("collect.protocol", "tcp")
	viper.SetDefault("collect.output", "/tmp/magellan/data/")
	viper.SetDefault("collect.force-update", false)
	viper.SetDefault("collect.cacert", "")
	viper.SetDefault("update.username", "")
	viper.SetDefault("update.password", "")
	viper.SetDefault("update.transfer-protocol", "https")
	viper.SetDefault("update.protocol", "tcp")
	viper.SetDefault("update.firmware.url", "")
	viper.SetDefault("update.firmware.version", "")
	viper.SetDefault("update.component", "")
	viper.SetDefault("update.status", false)

}
