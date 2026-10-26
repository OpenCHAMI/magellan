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
	"strings"

	"github.com/openchami/magellan/internal/format"
	logger "github.com/openchami/magellan/internal/log"
	"github.com/openchami/magellan/internal/util"
	_ "github.com/openchami/magellan/pkg/bmc/vendors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	logLevel    logger.LogLevel  = logger.INFO
	logFormat   logger.LogFormat = logger.FORMAT_JSON
	logColor    logger.LogColor  = logger.COLOR_AUTO
	logFile     string
	tokenPath   string
)

// The `root` command doesn't do anything on it's own except display
// a help message and then exits.
var rootCmd = &cobra.Command{
	Use:   "magellan",
	Short: "Redfish-based BMC discovery tool",
	Long: `Redfish-based BMC discovery tool with dynamic discovery features.
	
See 'magellan(1)' on getting started and examples.
	`,
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
		log.Debug().Str("path", logFile).Msg("closing log file")
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
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Configuration is loaded first so that --log-level/--log-format/
		// --log-color/--log-file and their LOG_* / config-file counterparts
		// are fully resolved before the logger is built. Any diagnostics
		// collected while configuration loads are replayed through the
		// configured logger afterwards (see emitConfigDiagnostics), so they
		// honor the selected format, color, log file, and level.
		InitializeConfig()
		InitializeLogger()
		emitConfigDiagnostics()
		resolveFlagsFromViper(cmd)
		return nil
	}
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "j", -1, "Set the number of concurrent processes")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 5, "Set the timeout for requests in seconds")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "Set the config file path")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "Set the access token")
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache", fmt.Sprintf("/tmp/%s/magellan/assets.db", util.GetCurrentUsername()), "Set the scanning result cache path")
	rootCmd.PersistentFlags().VarP(&logLevel, "log-level", "l", "Set the logger log-level (debug|info|warn|error|trace|disabled)")
	rootCmd.PersistentFlags().VarP(&logFormat, "log-format", "", "Set the log message format (json|rfc3339|basic)")
	rootCmd.PersistentFlags().VarP(&logColor, "log-color", "", "Set log color output (auto|on|off)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Set the path to store a log file")

	checkRegisterFlagCompletionError(rootCmd.RegisterFlagCompletionFunc("log-format", completionLogFormat))
	checkRegisterFlagCompletionError(rootCmd.RegisterFlagCompletionFunc("log-color", completionLogColor))
}

func checkBindFlagError(err error) {
	if err != nil {
		addDiagnostic(logger.WARN, err, "failed to bind cobra/viper flag", nil)
	}
}

func checkRegisterFlagCompletionError(err error) {
	if err != nil {
		log.Warn().Err(err).Msg("failed to register completion function")
	}
}

func helpMapToSlice(help map[string]string) []string {
	var helpSlice []string
	for k, v := range help {
		helpSlice = append(helpSlice, fmt.Sprintf("%s\t%s", k, v))
	}
	return helpSlice
}

// completionFormatData is the cobra completion function for any flag that uses
// the format.DataFormat type.
func completionFormatData(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return helpMapToSlice(format.DataFormatHelpMap), cobra.ShellCompDirectiveDefault
}

// completionLogFormat is the cobra completion function for the --log-format
// flag, which uses the logger.LogFormat type.
func completionLogFormat(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return helpMapToSlice(logger.LogFormatHelpMap), cobra.ShellCompDirectiveDefault
}

// completionLogColor is the cobra completion function for the --log-color
// flag, which uses the logger.LogColor type.
func completionLogColor(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return helpMapToSlice(logger.LogColorHelpMap), cobra.ShellCompDirectiveDefault
}

// diagnostic is a log message collected while configuration is being loaded,
// before the final logger has been constructed. Replaying diagnostics after
// InitializeLogger ensures they honor the configured format, color, log file,
// and level instead of leaking through zerolog's default JSON-to-stderr
// logger (see ochami#167 for the same pattern).
type diagnostic struct {
	level  logger.LogLevel
	err    error
	msg    string
	fields map[string]string
}

// configDiagnostics accumulates diagnostics emitted during configuration
// loading. It is drained by emitConfigDiagnostics once the logger is ready.
var configDiagnostics []diagnostic

// addDiagnostic records a diagnostic to be emitted after the logger is
// initialized. fields may be nil.
func addDiagnostic(level logger.LogLevel, err error, msg string, fields map[string]string) {
	configDiagnostics = append(configDiagnostics, diagnostic{
		level:  level,
		err:    err,
		msg:    msg,
		fields: fields,
	})
}

// emitConfigDiagnostics replays the diagnostics collected while configuration
// was loaded through the now-configured global logger, then clears them. This
// runs after InitializeLogger so that --log-format, --log-color, --log-file,
// and the resolved --log-level/LOG_LEVEL all apply.
func emitConfigDiagnostics() {
	for _, d := range configDiagnostics {
		var evt *zerolog.Event
		switch d.level {
		case logger.DEBUG:
			evt = log.Debug()
		case logger.WARN:
			evt = log.Warn()
		case logger.ERROR:
			evt = log.Error()
		default:
			evt = log.Info()
		}
		if d.err != nil {
			evt = evt.Err(d.err)
		}
		for k, v := range d.fields {
			evt = evt.Str(k, v)
		}
		evt.Msg(d.msg)
	}
	configDiagnostics = nil
}

// InitializeConfig() initializes a new config object by loading it
// from a file given a non-empty string.
func InitializeConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	for _, key := range []string{"concurrency", "timeout", "log-level", "log-format", "log-color", "access-token", "cache"} {
		f := rootCmd.PersistentFlags().Lookup(key)
		viper.SetDefault(key, f.DefValue)
		if f.Changed {
			checkBindFlagError(viper.BindPFlag(key, f))
		}
	}

	if configPath == "" {
		config_dir := os.Getenv("XDG_CONFIG_HOME")
		if config_dir == "" {
			config_dir = "$HOME/.config"
		}
		viper.AddConfigPath(config_dir + "/magellan.yaml")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
		// File type left unspecified; Viper will auto-parse based on extension
		// e.g. ~/.config/magellan/config.yaml will parse as YAML
	} else {
		viper.SetConfigFile(configPath)
	}
	if err := viper.ReadInConfig(); err != nil {
		addDiagnostic(logger.DEBUG, err, "failed to load config", nil)
	}

	for _, key := range []string{"concurrency", "timeout", "log-level", "log-format", "log-color", "access-token", "cache"} {
		f := rootCmd.PersistentFlags().Lookup(key)
		if f.Changed || !viper.IsSet(key) {
			continue
		}
		switch key {
		case "concurrency":
			concurrency = viper.GetInt(key)
		case "timeout":
			timeout = viper.GetInt(key)
		case "log-level":
			if err := logLevel.Set(viper.GetString(key)); err != nil {
				addDiagnostic(logger.WARN, err, "failed to set flag from config", map[string]string{"key": key})
			}
		case "log-format":
			if err := logFormat.Set(viper.GetString(key)); err != nil {
				addDiagnostic(logger.WARN, err, "failed to set flag from config", map[string]string{"key": key})
			}
		case "log-color":
			if err := logColor.Set(viper.GetString(key)); err != nil {
				addDiagnostic(logger.WARN, err, "failed to set flag from config", map[string]string{"key": key})
			}
		case "access-token":
			accessToken = viper.GetString(key)
		case "cache":
			cachePath = viper.GetString(key)
		}
	}
}

func InitializeLogger() {
	// initialize the logger
	err := logger.Init(logLevel, logFormat, logColor, logFile)
	if err != nil {
		log.Error().Err(err).Msg("failed to initialize logger")
		os.Exit(1)
	}
}

var viperKeyAliases = map[string]string{
	"scan.subnet":      "scan.subnets",
	"scan.port":        "scan.ports",
	"scan.subnet-mask": "scan.subnet-masks",
}

func resolveFlagsFromViper(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			return
		}
		key := cmd.Name() + "." + f.Name
		keys := []string{key}
		if alias, ok := viperKeyAliases[key]; ok {
			keys = []string{alias, key}
		}
		for parent := cmd.Parent(); parent != nil && parent != rootCmd; parent = parent.Parent() {
			keys = append(keys, parent.Name()+"."+f.Name)
		}
		for _, k := range keys {
			if viper.IsSet(k) {
				setFlagFromViper(cmd, f, k)
				return
			}
		}
	})
}

func setFlagFromViper(cmd *cobra.Command, f *pflag.Flag, key string) {
	switch f.Value.Type() {
	case "stringSlice":
		if vals := viper.GetStringSlice(key); len(vals) > 0 {
			if err := cmd.Flags().Set(f.Name, strings.Join(vals, ",")); err != nil {
				log.Warn().Err(err).Str("flag", f.Name).Msg("failed to set flag from config")
			}
		}
	default:
		if s := viper.GetString(key); s != "" && s != f.DefValue {
			if err := cmd.Flags().Set(f.Name, s); err != nil {
				log.Warn().Err(err).Str("flag", f.Name).Msg("failed to set flag from config")
			}
		}
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
	viper.SetDefault("power.cacert", "")
	viper.SetDefault("power.vendor", "")
	viper.SetDefault("server.host", "")
	viper.SetDefault("server.port", 8500)
	viper.SetDefault("server.tls-cert", "")
	viper.SetDefault("server.tls-key", "")
	viper.SetDefault("server.auth-token", "")
	viper.SetDefault("server.vendor", "")
	viper.SetDefault("server.secrets-file", "")
	viper.SetDefault("server.insecure", false)
}
