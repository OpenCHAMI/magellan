package cmd

import (
	magellan "github.com/bikeshack/magellan/internal"
	"github.com/bikeshack/magellan/internal/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	firmwarePath string
	firmwareVersion string
	component string
)

var updateCmd = &cobra.Command{
	Use: "update",
	Short: "Update BMC node firmware",
	Run: func(cmd *cobra.Command, args []string) {
		l := log.NewLogger(logrus.New(), logrus.DebugLevel)
		q := &magellan.UpdateParams {
			FirmwarePath: firmwarePath,
			FirmwareVersion: firmwareVersion,
			Component: component,
			QueryParams: magellan.QueryParams{
				User: user,
				Pass: pass,
				Timeout: timeout,
			},
		}
		client, err := magellan.NewClient(l, &q.QueryParams)
		if err != nil {
			l.Log.Errorf("could not make client: %v", err)
		}
		err = magellan.UpdateFirmware(client, l, q)
		if err != nil {
			l.Log.Errorf("could not update firmware: %v", err)
		}
	},
}

func init() {
	updateCmd.PersistentFlags().StringVar(&user, "user", "", "set the BMC user")
	updateCmd.PersistentFlags().StringVar(&pass, "pass", "", "set the BMC password")
	updateCmd.PersistentFlags().StringVar(&firmwarePath, "firmware-path", "", "set the path to the firmware")
	updateCmd.PersistentFlags().StringVar(&firmwareVersion, "firmware-version", "", "set the version of firmware to be installed")
	updateCmd.PersistentFlags().StringVar(&component, "component", "", "set the component to upgrade")
	rootCmd.AddCommand(updateCmd)
}