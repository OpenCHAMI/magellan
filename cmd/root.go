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

	magellan "github.com/OpenCHAMI/magellan/internal"
	logger "github.com/OpenCHAMI/magellan/internal/log"
	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CLI arguments as variables to not fiddle with error-prone strings
var (
	accessToken string
	timeout     int
	concurrency int
	ports       []int
	protocol    string
	cacertPath  string
	username    string
	password    string
	cachePath   string
	outputPath  string
	outputDir   string
	configPath  string
	showOutput  bool
	forceUpdate bool
	insecure    bool
	idMap       string
	logLevel    logger.LogLevel = logger.INFO
	logFile     string
)

// The `root` command doesn't do anything on it's own except display
// a help message and then exits.
var rootCmd = &cobra.Command{
	Use:   "magellan",
	Short: "Redfish-based BMC discovery tool",
	Long:  "Redfish-based BMC discovery tool with dynamic discovery features.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// initialize the logger
		err := logger.InitWithLogLevel(logLevel, logFile)
		if err != nil {
			log.Error().Err(err).Msg("failed to initialize logger")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			err := cmd.Help()
			if err != nil {
				log.Error().Err(err).Msg("failed to print help")
			}
			os.Exit(0)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		log.Debug().Msg("closing log file")
		err := logger.LogFile.Close()
		if err != nil {
			log.Error().Err(err).Msg("failed to close log file")
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
	cobra.OnInitialize(InitializeConfig)
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "j", -1, "Set the number of concurrent processes")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 5, "Set the timeout for requests in seconds")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Set the config file path")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "Set the access token")
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", util.GetCurrentUsername()), "Set the scanning result cache path")
	rootCmd.PersistentFlags().VarP(&logLevel, "log-level", "l", "Set the logger log-level (debug|info|warn|error|trace|disabled)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Set the path to store a log file")

	// bind viper config flags with cobra
	checkBindFlagError(viper.BindPFlag("concurrency", rootCmd.PersistentFlags().Lookup("concurrency")))
	checkBindFlagError(viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout")))
	checkBindFlagError(viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level")))
	checkBindFlagError(viper.BindPFlag("access-token", rootCmd.PersistentFlags().Lookup("access-token")))
	checkBindFlagError(viper.BindPFlag("cache", rootCmd.PersistentFlags().Lookup("cache")))

}

func checkBindFlagError(err error) {
	if err != nil {
		log.Error().Err(err).Msg("failed to bind cobra/viper flag")
	}
}

// InitializeConfig() initializes a new config object by loading it
// from a file given a non-empty string.
func InitializeConfig() {
	viper.AutomaticEnv()
	if configPath == "" {
		config_dir := os.Getenv("XDG_CONFIG_HOME")
		if config_dir == "" {
			config_dir = "$HOME/.config"
		}
		viper.AddConfigPath(config_dir + "/magellan")
		viper.SetConfigName("config")
		// File type left unspecified; Viper will auto-parse based on extension
		// e.g. ~/.config/magellan/config.yaml will parse as YAML
	} else {
		viper.SetConfigFile(configPath)
	}
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			err = fmt.Errorf("config file not found: %w", err)
		} else {
			err = fmt.Errorf("failed to load config file: %w", err)
		}
		log.Error().Err(err).Msg("failed to load config")
	}
}

// SetDefaults() resets all of the viper properties back to their
// default values.
//
// TODO: This function should probably be moved to 'internal/config.go'
// instead of in this file.
func SetDefaults() {
	viper.SetDefault("threads", 1)
	viper.SetDefault("timeout", 5)
	viper.SetDefault("config", "")
	viper.SetDefault("verbose", false)
	viper.SetDefault("debug", false)
	viper.SetDefault("cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", util.GetCurrentUsername()))
	viper.SetDefault("scan.hosts", []string{})
	viper.SetDefault("scan.ports", []int{})
	viper.SetDefault("scan.subnets", []string{})
	viper.SetDefault("scan.subnet-masks", []net.IP{})
	viper.SetDefault("scan.disable-probing", false)
	viper.SetDefault("scan.disable-cache", false)
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
<<<<<<< HEAD
	viper.SetDefault("power.cacert", "")
=======
>>>>>>> a36795f (refactor: updated format and log usage)
}
