package cmd

import (
	"github.com/OpenCHAMI/magellan/pkg/bmc"
	"github.com/spf13/cobra"
)

var ConfigureCmd = &cobra.Command{
	Use: "configure",
	Example: `
  # get list all of the settings available to update 
  magellan settings list
  
  # get list of all of the Oem settings 
  magellan settings list Oem
  
  # get the port number for IPMI
  magellan settings get IPMI Port 
  
  # update the setting
  magellan settings update IPMI Port 623
`,
	Short: ``,
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var ConfigureListCmd = &cobra.Command{
	Use:   "list",
	Short: ``,
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		gofish
	},
}

var ConfigureGetCmd = &cobra.Command{
	Use:   "get",
	Short: ``,
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var ConfigureSetCmd = &cobra.Command{
	Use:   "set",
	Short: ``,
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			config = bmc.Config{
				URI:             uri,
				CredentialStore: store,
				Insecure:        insecure,
				UseDefault:      true,
			}
		)
		bmc.SetProperty(config, args)
	},
}

func init() {
	ConfigureCmd.AddCommand(ConfigureListCmd, ConfigureGetCmd, ConfigureSetCmd)

	rootCmd.AddCommand(ConfigureCmd)
}
