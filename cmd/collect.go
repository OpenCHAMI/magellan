package cmd

import (
	"fmt"

	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/api/smd"
	"github.com/OpenCHAMI/magellan/internal/db/sqlite"
	"github.com/OpenCHAMI/magellan/internal/log"
	"github.com/cznic/mathutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	forceUpdate bool
)

var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Query information about BMC",
	Run: func(cmd *cobra.Command, args []string) {
		// make application logger
		l := log.NewLogger(logrus.New(), logrus.DebugLevel)

		// get probe states stored in db from scan
		probeStates, err := sqlite.GetProbeResults(dbpath)
		if err != nil {
			l.Log.Errorf("could not get states: %v", err)
		}

		// try to load access token either from env var, file, or config if var not set
		if accessToken == "" {
			var err error
			accessToken, err = LoadAccessToken()
			if err != nil {
				l.Log.Errorf("failed to load access token: %v", err)
			}
		}

		if verbose {
			fmt.Printf("access token: %v\n", accessToken)
		}

		//
		if threads <= 0 {
			threads = mathutil.Clamp(len(probeStates), 1, 255)
		}
		q := &magellan.QueryParams{
			User:        user,
			Pass:        pass,
			Protocol:    protocol,
			Drivers:     drivers,
			Preferred:   preferredDriver,
			Timeout:     timeout,
			Threads:     threads,
			Verbose:     verbose,
			CaCertPath:  cacertPath,
			OutputPath:  outputPath,
			ForceUpdate: forceUpdate,
			AccessToken: accessToken,
		}
		magellan.CollectAll(&probeStates, l, q)

		// confirm the inventories were added
		err = smd.GetRedfishEndpoints()
		if err != nil {
			l.Log.Errorf("could not get redfish endpoints: %v", err)
		}
	},
}

func init() {
	collectCmd.PersistentFlags().StringSliceVar(&drivers, "driver", []string{"redfish"}, "set the driver(s) and fallback drivers to use")
	collectCmd.PersistentFlags().StringVar(&smd.Host, "host", smd.Host, "set the host to the smd API")
	collectCmd.PersistentFlags().IntVarP(&smd.Port, "port", "p", smd.Port, "set the port to the smd API")
	collectCmd.PersistentFlags().StringVar(&user, "user", "", "set the BMC user")
	collectCmd.PersistentFlags().StringVar(&pass, "pass", "", "set the BMC password")
	collectCmd.PersistentFlags().StringVar(&protocol, "protocol", "https", "set the Redfish protocol")
	collectCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", "/tmp/magellan/data/", "set the path to store collection data")
	collectCmd.PersistentFlags().BoolVar(&forceUpdate, "force-update", false, "set flag to force update data sent to SMD ")
	collectCmd.PersistentFlags().StringVar(&preferredDriver, "preferred-driver", "ipmi", "set the preferred driver to use")
	collectCmd.PersistentFlags().StringVar(&ipmitoolPath, "ipmitool.path", "/usr/bin/ipmitool", "set the path for ipmitool")
	collectCmd.PersistentFlags().StringVar(&cacertPath, "ca-cert", "", "path to CA cert. (defaults to system CAs)")

	viper.BindPFlag("collect.driver", collectCmd.Flags().Lookup("driver"))
	viper.BindPFlag("collect.host", collectCmd.Flags().Lookup("host"))
	viper.BindPFlag("collect.port", collectCmd.Flags().Lookup("port"))
	viper.BindPFlag("collect.user", collectCmd.Flags().Lookup("user"))
	viper.BindPFlag("collect.pass", collectCmd.Flags().Lookup("pass"))
	viper.BindPFlag("collect.protocol", collectCmd.Flags().Lookup("protocol"))
	viper.BindPFlag("collect.output", collectCmd.Flags().Lookup("output"))
	viper.BindPFlag("collect.force-update", collectCmd.Flags().Lookup("force-update"))
	viper.BindPFlag("collect.preferred-driver", collectCmd.Flags().Lookup("preferred-driver"))
	viper.BindPFlag("collect.ipmitool.path", collectCmd.Flags().Lookup("ipmitool.path"))
	viper.BindPFlag("collect.secure-tls", collectCmd.Flags().Lookup("secure-tls"))
	viper.BindPFlag("collect.cert-pool", collectCmd.Flags().Lookup("cert-pool"))

	rootCmd.AddCommand(collectCmd)
}
