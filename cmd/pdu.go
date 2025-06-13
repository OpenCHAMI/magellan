package cmd

import (
	"github.com/spf13/cobra"
)

var PduCmd = &cobra.Command{
	Use:   "pdu",
	Short: "Perform actions on Power Distribution Units (PDUs)",
	Long:  `A collection of commands to discover and manage PDUs that may not use the Redfish protocol.`,
}

func init() {
	rootCmd.AddCommand(PduCmd)
}
