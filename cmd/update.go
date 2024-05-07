package cmd

import (
	magellan "github.com/OpenCHAMI/magellan/internal"
	"github.com/OpenCHAMI/magellan/internal/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	host             string
	port             int
	firmwareUrl      string
	firmwareVersion  string
	component        string
	transferProtocol string
	status           bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update BMC node firmware",
	Run: func(cmd *cobra.Command, args []string) {
		l := log.NewLogger(logrus.New(), logrus.DebugLevel)
		q := &magellan.UpdateParams{
			FirmwarePath:     firmwareUrl,
			FirmwareVersion:  firmwareVersion,
			Component:        component,
			TransferProtocol: transferProtocol,
			QueryParams: magellan.QueryParams{
				Drivers:   []string{"redfish"},
				Preferred: "redfish",
				Protocol:  protocol,
				Host:      host,
				User:      user,
				Pass:      pass,
				Timeout:   timeout,
				Port:      port,
			},
		}

		// check if required params are set
		if host == "" || user == "" || pass == "" {
			l.Log.Fatal("requires host, user, and pass to be set")
		}

		// get status if flag is set and exit
		if status {
			err := magellan.GetUpdateStatus(q)
			if err != nil {
				l.Log.Errorf("could not get update status: %v", err)
			}
			return
		}

		// client, err := magellan.NewClient(l, &q.QueryParams)
		// if err != nil {
		// 	l.Log.Errorf("could not make client: %v", err)
		// }
		// err = magellan.UpdateFirmware(client, l, q)
		err := magellan.UpdateFirmwareRemote(q)
		if err != nil {
			l.Log.Errorf("could not update firmware: %v", err)
		}
	},
}

func init() {
	updateCmd.Flags().StringVar(&host, "bmc-host", "", "set the BMC host")
	updateCmd.Flags().IntVar(&port, "bmc-port", 443, "set the BMC port")
	updateCmd.Flags().StringVar(&user, "user", "", "set the BMC user")
	updateCmd.Flags().StringVar(&pass, "pass", "", "set the BMC password")
	updateCmd.Flags().StringVar(&transferProtocol, "transfer-protocol", "HTTP", "set the transfer protocol")
	updateCmd.Flags().StringVar(&protocol, "protocol", "https", "set the Redfish protocol")
	updateCmd.Flags().StringVar(&firmwareUrl, "firmware-url", "", "set the path to the firmware")
	updateCmd.Flags().StringVar(&firmwareVersion, "firmware-version", "", "set the version of firmware to be installed")
	updateCmd.Flags().StringVar(&component, "component", "", "set the component to upgrade")
	updateCmd.Flags().BoolVar(&status, "status", false, "get the status of the update")

	viper.BindPFlag("bmc-host", updateCmd.Flags().Lookup("bmc-host"))
	viper.BindPFlag("bmc-port", updateCmd.Flags().Lookup("bmc-port"))
	viper.BindPFlag("user", updateCmd.Flags().Lookup("user"))
	viper.BindPFlag("pass", updateCmd.Flags().Lookup("pass"))
	viper.BindPFlag("transfer-protocol", updateCmd.Flags().Lookup("transfer-protocol"))
	viper.BindPFlag("protocol", updateCmd.Flags().Lookup("protocol"))
	viper.BindPFlag("firmware-url", updateCmd.Flags().Lookup("firmware-url"))
	viper.BindPFlag("firmware-version", updateCmd.Flags().Lookup("firmware-version"))
	viper.BindPFlag("component", updateCmd.Flags().Lookup("component"))
	viper.BindPFlag("secure-tls", updateCmd.Flags().Lookup("secure-tls"))
	viper.BindPFlag("status", updateCmd.Flags().Lookup("status"))

	rootCmd.AddCommand(updateCmd)
}
