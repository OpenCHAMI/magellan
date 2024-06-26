package cmd

import (
	"fmt"
	"net"
	"os"
	"os/user"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/api/smd"
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
	testToken := os.Getenv("MAGELLAN_ACCESS_TOKEN")
	if testToken != "" {
		return testToken, nil
	}

	// try reading access token from a file
	b, err := os.ReadFile(tokenPath)
	if err == nil {
		return string(b), nil
	}

	// TODO: try to load token from config
	testToken = viper.GetString("access_token")
	if testToken != "" {
		return testToken, nil
	}
	return "", fmt.Errorf("failed toload token from environment variable, file, or config")
}

func init() {
	currentUser, _ = user.Current()
	cobra.OnInitialize(InitializeConfig)
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", -1, "set the number of concurrent processes")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "set the timeout")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "set the config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "set output verbosity")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "set the access token")
	rootCmd.PersistentFlags().StringVar(&cachePath, "cache", fmt.Sprintf("/tmp/%smagellan/magellan.db", currentUser.Username+"/"), "set the scanning result cache path")

	// bind viper config flags with cobra
	viper.BindPFlag("concurrency", rootCmd.Flags().Lookup("concurrency"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("verbose", rootCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("cache", rootCmd.Flags().Lookup("cache"))
	viper.BindPFlags(rootCmd.Flags())
}

func InitializeConfig() {
	if configPath != "" {
		magellan.LoadConfig(configPath)
	}
}

func SetDefaults() {
	viper.SetDefault("threads", 1)
	viper.SetDefault("timeout", 30)
	viper.SetDefault("config", "")
	viper.SetDefault("verbose", false)
	viper.SetDefault("cache", "/tmp/magellan/magellan.db")
	viper.SetDefault("scan.hosts", []string{})
	viper.SetDefault("scan.ports", []int{})
	viper.SetDefault("scan.subnets", []string{})
	viper.SetDefault("scan.subnet-masks", []net.IP{})
	viper.SetDefault("scan.disable-probing", false)
	viper.SetDefault("collect.driver", []string{"redfish"})
	viper.SetDefault("collect.host", smd.Host)
	viper.SetDefault("collect.port", smd.Port)
	viper.SetDefault("collect.user", "")
	viper.SetDefault("collect.pass", "")
	viper.SetDefault("collect.protocol", "https")
	viper.SetDefault("collect.output", "/tmp/magellan/data/")
	viper.SetDefault("collect.force-update", false)
	viper.SetDefault("collect.ca-cert", "")
	viper.SetDefault("bmc-host", "")
	viper.SetDefault("bmc-port", 443)
	viper.SetDefault("user", "")
	viper.SetDefault("pass", "")
	viper.SetDefault("transfer-protocol", "HTTP")
	viper.SetDefault("protocol", "https")
	viper.SetDefault("firmware-url", "")
	viper.SetDefault("firmware-version", "")
	viper.SetDefault("component", "")
	viper.SetDefault("secure-tls", false)
	viper.SetDefault("status", false)

}
