package cmd

import (
	"fmt"
	"os/user"

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
			l.Log.Errorf("failed toget states: %v", err)
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
		if concurrency <= 0 {
			concurrency = mathutil.Clamp(len(probeStates), 1, 255)
		}
		q := &magellan.QueryParams{
			User:        username,
			Pass:        password,
			Protocol:    protocol,
			Timeout:     timeout,
			Concurrency: concurrency,
			Verbose:     verbose,
			CaCertPath:  cacertPath,
			OutputPath:  outputPath,
			ForceUpdate: forceUpdate,
			AccessToken: accessToken,
		}
		magellan.CollectAll(&probeStates, l, q)

		// add necessary headers for final request (like token)
		headers := make(map[string]string)
		if q.AccessToken != "" {
			headers["Authorization"] = "Bearer " + q.AccessToken
		}
	},
}

func init() {
	currentUser, _ = user.Current()
	collectCmd.PersistentFlags().StringVar(&smd.Host, "host", smd.Host, "set the host to the SMD API")
	collectCmd.PersistentFlags().IntVarP(&smd.Port, "port", "p", smd.Port, "set the port to the SMD API")
	collectCmd.PersistentFlags().StringVar(&username, "user", "", "set the BMC user")
	collectCmd.PersistentFlags().StringVar(&password, "pass", "", "set the BMC password")
	collectCmd.PersistentFlags().StringVar(&protocol, "protocol", "https", "set the protocol used to query")
	collectCmd.PersistentFlags().StringVarP(&outputPath, "output", "o", fmt.Sprintf("/tmp/%smagellan/data/", currentUser.Username+"/"), "set the path to store collection data")
	collectCmd.PersistentFlags().BoolVar(&forceUpdate, "force-update", false, "set flag to force update data sent to SMD")
	collectCmd.PersistentFlags().StringVar(&cacertPath, "ca-cert", "", "path to CA cert. (defaults to system CAs)")
	collectCmd.MarkFlagsRequiredTogether("user", "pass")

	viper.BindPFlag("collect.driver", collectCmd.Flags().Lookup("driver"))
	viper.BindPFlag("collect.host", collectCmd.Flags().Lookup("host"))
	viper.BindPFlag("collect.port", collectCmd.Flags().Lookup("port"))
	viper.BindPFlag("collect.user", collectCmd.Flags().Lookup("user"))
	viper.BindPFlag("collect.pass", collectCmd.Flags().Lookup("pass"))
	viper.BindPFlag("collect.protocol", collectCmd.Flags().Lookup("protocol"))
	viper.BindPFlag("collect.output", collectCmd.Flags().Lookup("output"))
	viper.BindPFlag("collect.force-update", collectCmd.Flags().Lookup("force-update"))
	viper.BindPFlag("collect.ca-cert", collectCmd.Flags().Lookup("secure-tls"))
	viper.BindPFlags(collectCmd.Flags())

	rootCmd.AddCommand(collectCmd)
}
