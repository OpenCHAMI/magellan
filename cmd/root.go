package cmd

import (
	"fmt"
	"net"
	"os"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/api/smd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	configPath      string
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
	return "", fmt.Errorf("could not load token from environment variable, file, or config")
}

func init() {
	cobra.OnInitialize(InitializeConfig)
	rootCmd.PersistentFlags().IntVar(&threads, "threads", -1, "set the number of threads")
	rootCmd.PersistentFlags().IntVar(&timeout, "timeout", 30, "set the timeout")
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "set the config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "set verbose flag")
	rootCmd.PersistentFlags().StringVar(&accessToken, "access-token", "", "set the access token")
	rootCmd.PersistentFlags().StringVar(&dbpath, "db.path", "/tmp/magellan/magellan.db", "set the probe storage path")

	// bind viper config flags with cobra
	viper.BindPFlag("threads", rootCmd.Flags().Lookup("threads"))
	viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	viper.BindPFlag("verbose", rootCmd.Flags().Lookup("verbose"))
	viper.BindPFlag("db.path", rootCmd.Flags().Lookup("db.path"))
	// viper.BindPFlags(rootCmd.Flags())
}

func InitializeConfig() {
	if configPath != "" {
		magellan.LoadConfig(configPath)
		fmt.Printf("subnets: %v\n", viper.Get("scan.subnets"))
	}
}

func SetDefaults() {
	viper.SetDefault("threads", 1)
	viper.SetDefault("timeout", 30)
	viper.SetDefault("config", "")
	viper.SetDefault("verbose", false)
	viper.SetDefault("db.path", "/tmp/magellan/magellan.db")
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
	viper.SetDefault("collect.preferred-driver", "ipmi")
	viper.SetDefault("collect.ipmitool.path", "/usr/bin/ipmitool")
	viper.SetDefault("collect.secure-tls", false)
	viper.SetDefault("collect.cert-pool", "")
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
