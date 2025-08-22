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

	"github.com/OpenCHAMI/magellan/internal/util"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	// Load access token from file, if path is provided
	if viper.IsSet("token-path") {
		b, err := os.ReadFile(viper.GetString("token-path"))
		if err == nil {
			viper.Set("access-token", string(b))
		} else {
			log.Warn().Err(err).Msg("failed to load access token from file; continuing without it")
		}
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(InitializeConfig)

	addFlag("concurrency", rootCmd, "concurrency", "j", -1, "Set the number of concurrent processes")
	addFlag("timeout", rootCmd, "timeout", "t", 5, "Set the timeout for requests in seconds")
	addFlag("config", rootCmd, "config", "c", "", "Set the config file path")
	addFlag("verbose", rootCmd, "verbose", "v", false, "Set to enable/disable verbose output")
	addFlag("debug", rootCmd, "debug", "", false, "Set to enable/disable debug messages")
	addFlag("access-token", rootCmd, "access-token", "", "", "Set the access token")
	checkBindFlagError(viper.BindEnv("access-token", "ACCESS_TOKEN"))
	addFlag("token-path", rootCmd, "token-path", "", ".ochami-token", "Set the path to load/save the access token")
	addFlag("cache", rootCmd, "cache", "", fmt.Sprintf("/tmp/%s/magellan/assets.db", util.GetCurrentUsername()), "Set the scanning result cache path")
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
	if viper.IsSet("config") {
		viper.SetConfigFile(viper.GetString("config"))
	} else {
		config_dir := os.Getenv("XDG_CONFIG_HOME")
		if config_dir == "" {
			config_dir = "$HOME/.config"
		}
		viper.AddConfigPath(config_dir + "/magellan")
		viper.SetConfigName("config")
		// File type left unspecified; Viper will auto-parse based on extension
		// e.g. ~/.config/magellan/config.yaml will parse as YAML
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

// Add a flag to `cmd` which is bound to `configKey` in Viper. The flag's type is inferred from `defaultValue`.
func addFlag(configKey string, cmd *cobra.Command, flagName string, flagShort string, defaultValue any, usage string) {
	switch defaultValue := defaultValue.(type) {
	case bool:
		cmd.PersistentFlags().BoolVarP(new(bool), flagName, flagShort, defaultValue, usage)
	case int:
		cmd.PersistentFlags().IntVarP(new(int), flagName, flagShort, defaultValue, usage)
	case []int:
		cmd.PersistentFlags().IntSliceVarP(new([]int), flagName, flagShort, defaultValue, usage)
	case string:
		cmd.PersistentFlags().StringVarP(new(string), flagName, flagShort, defaultValue, usage)
	case []string:
		cmd.PersistentFlags().StringArrayVarP(new([]string), flagName, flagShort, defaultValue, usage)
	case net.IPMask:
		cmd.PersistentFlags().IPMaskVarP(new(net.IPMask), flagName, flagShort, defaultValue, usage)
	default:
		log.Fatal().Msgf("unhandled flag type '%T', cannot add flag of that type", defaultValue)
		// Calls os.Exit() for us
	}
	checkBindFlagError(viper.BindPFlag(configKey, cmd.PersistentFlags().Lookup(flagName)))
}
