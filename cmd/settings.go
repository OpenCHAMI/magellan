package cmd

import (
	"fmt"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	settingsFormat         format.DataFormat = format.FORMAT_JSON
	settingsInputFormat    format.DataFormat = format.FORMAT_YAML
	settingsInventoryFile  string
	settingsPreserveConfig string
)

var SettingsCmd = &cobra.Command{
	Use: "settings",
	Example: `  # list the setting categories present on a BMC
  magellan settings list 172.16.0.105

  # list the network protocols present on a BMC
  magellan settings list 172.16.0.105 NetworkProtocol

  # get a specific network setting from a BMC
  magellan settings get 172.16.0.105 NetworkProtocol SSH

  # get a nested computer system property
  magellan settings get 172.16.0.105 ComputerSystem Boot BootOrder

  # set a network setting
  magellan settings set 172.16.0.105 NetworkProtocol SSH '{"ProtocolEnabled":true,"Port":22}'

  # get the first ethernet interface's IP settings
  magellan settings get 172.16.0.105 EthernetInterface 0

  # factory reset the BMC manager
  magellan settings reset 172.16.0.105

  # factory reset preserving network settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork`,
	Short: "Configure BMC properties through Redfish",
	Long: `Configure BMC properties through Redfish.

Supported categories:
  NetworkProtocol    SSH, HTTPS, IPMI, NTP, SNMP, and other network services
  EthernetInterface  IP addresses, MAC addresses, DHCP settings
  ComputerSystem     Boot order, asset tag, and other system-level settings
  Manager            Firmware version, model, and other manager properties
  Accounts           BMC user accounts, roles, and access control

See 'magellan-settings(1)' for more details. See 'magellan(1)' for a list of
available environment variables.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			log.Error().Err(err).Msg("failed to print help")
		}
	},
}

var SettingsListCmd = &cobra.Command{
	Use:   "list <node> [<category> [<item> [<property>...]]]",
	Short: "List setting categories, items, or properties available on a BMC",
	Long: `List setting categories, items, or properties available on a BMC.

When called with just a node, lists the setting categories present on that BMC.
When called with a category, lists the items present under that category.
When called with a category and item, lists the properties available on that item.
Additional property arguments walk deeper into nested structures.`,
	Example: `  # list all available categories on a BMC
  magellan settings list 172.16.0.105

  # list the network protocols present on a BMC
  magellan settings list 172.16.0.105 NetworkProtocol

  # list the properties of the SSH protocol
  magellan settings list 172.16.0.105 NetworkProtocol SSH

  # walk deeper into a nested structure
  magellan settings list 172.16.0.105 ComputerSystem Node0 Boot`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]

		client, err := bmc.Connect(nodeArg, settingsInventoryFile, settingsInputFormat)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		out := cmd.OutOrStdout()

		// No category: list the categories present on the BMC.
		if len(args) == 1 {
			return bmc.ListSettingsCategories(client, out)
		}
		category := args[1]
		if _, ok := bmc.SettingsCategories[category]; !ok {
			return fmt.Errorf("unknown category %q; use 'magellan settings list <node>' to see available categories", category)
		}

		log.Info().
			Str("category", category).
			Str("description", bmc.SettingsCategories[category]).
			Send()

		// Category + no item: list the items present under the category.
		if len(args) == 2 {
			return bmc.ListSettingsItems(client, out, category)
		}

		// Category + item (+ optional property path): list the properties
		// available at the resolved item/path.
		return bmc.ListSettingsProperties(client, out, category, args[2], args[3:])
	},
}

var SettingsGetCmd = &cobra.Command{
	Use:   "get <node> <category> [item] [property...]",
	Short: "Get a BMC setting value",
	Long: `Get a BMC setting value by category and property path.

The path is walked into nested properties as deeply as the BMC schema allows.
The first item after the category selects the setting to read, and any
additional items walk deeper into nested properties. For NetworkProtocol, the
first item is the protocol name (e.g., SSH, HTTPS, IPMI, NTP). For
EthernetInterface, the first item is the interface index (0, 1, ...). For
ComputerSystem and Manager, the first item is a property name on the first
resource exposed by the BMC. For Accounts, the first item is the account ID.`,
	Example: `  # get SSH protocol settings
  magellan settings get 172.16.0.105 NetworkProtocol SSH

  # get the first ethernet interface's IPv4 address
  magellan settings get 172.16.0.105 EthernetInterface 0 IPv4Addresses

  # get a nested boot property
  magellan settings get 172.16.0.105 ComputerSystem Node0 Boot BootOrder

  # get all accounts
  magellan settings get 172.16.0.105 Accounts

  # get a specific account
  magellan settings get 172.16.0.105 Accounts 1`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]
		category := args[1]

		client, err := bmc.Connect(nodeArg, settingsInventoryFile, settingsInputFormat)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		if _, ok := bmc.SettingsCategories[category]; !ok {
			return fmt.Errorf("unknown category %q; use 'magellan settings list' to see available categories", category)
		}

		var result any
		if len(args) == 2 {
			// No item: return the whole category resource(s).
			result, err = bmc.ResolveCategoryCollection(client, category)
		} else {
			item, rErr := bmc.ResolveCategoryItem(client, category, args[2])
			if rErr != nil {
				return rErr
			}
			result = item
			if len(args) > 3 {
				resolved, wErr := bmc.ResolveSettingsPath(item, args[3:])
				if wErr != nil {
					return wErr
				}
				result = resolved
			}
		}
		if err != nil {
			return err
		}

		output, err := format.MarshalData(result, settingsFormat)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(output))
		return nil
	},
}

var SettingsSetCmd = &cobra.Command{
	Use:   "set <node> <category> <property> <value>",
	Short: "Set a BMC setting value",
	Long: `Set a BMC setting value by category, property, and value.

The value should be a JSON string for complex types or a simple string for
scalar values.`,
	Example: `  # enable SSH on the BMC
  magellan settings set 172.16.0.105 NetworkProtocol SSH '{"ProtocolEnabled":true,"Port":22}'

  # update the first ethernet interface IP
  magellan settings set 172.16.0.105 EthernetInterface 0 '{"IPv4Addresses":[{"Address":"172.16.0.105","SubnetMask":"255.255.255.0","Gateway":"172.16.0.1"}]}'`,
	Args: cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]
		category := args[1]
		property := args[2]
		value := args[3]

		client, err := bmc.Connect(nodeArg, settingsInventoryFile, settingsInputFormat)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		switch category {
		case "NetworkProtocol":
			if err := bmc.SetNetworkProtocol(client, property, value); err != nil {
				return fmt.Errorf("failed to set NetworkProtocol.%s: %w", property, err)
			}
		case "EthernetInterface":
			idx := 0
			if _, err := fmt.Sscanf(property, "%d", &idx); err != nil {
				return fmt.Errorf("invalid interface index %q: %w", property, err)
			}
			if err := bmc.SetEthernetInterface(client, idx, value); err != nil {
				return fmt.Errorf("failed to set EthernetInterface[%d]: %w", idx, err)
			}
		case "ComputerSystem":
			if err := bmc.SetComputerSystemProperty(client, property, value); err != nil {
				return fmt.Errorf("failed to set ComputerSystem.%s: %w", property, err)
			}
		case "Manager":
			if err := bmc.SetManagerProperty(client, property, value); err != nil {
				return fmt.Errorf("failed to set Manager.%s: %w", property, err)
			}
		case "Accounts":
			if err := bmc.UpdateAccount(client, property, value); err != nil {
				return fmt.Errorf("failed to update account %s: %w", property, err)
			}
		default:
			return fmt.Errorf("unknown category %q; use 'magellan settings list' to see available categories", category)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Successfully set %s.%s\n", category, property)
		return nil
	},
}

var SettingsResetCmd = &cobra.Command{
	Use:   "reset <node>",
	Short: "Factory reset the BMC manager",
	Long: `Factory reset the BMC manager.

By default, this resets all settings. Use --preserve-config to preserve
specific settings during the reset. If the BMC does not support resetting to
defaults via the Manager.ResetToDefaults action, or does not support the
requested preserve type, an error is reported before any reset is attempted.`,
	Example: `  # factory reset all settings
  magellan settings reset 172.16.0.105

  # factory reset but preserve network settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork

  # factory reset but preserve network and user settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetworkAndUsers`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		nodeArg := args[0]

		client, err := bmc.Connect(nodeArg, settingsInventoryFile, settingsInputFormat)
		if err != nil {
			return fmt.Errorf("failed to connect to BMC for node %q: %w", nodeArg, err)
		}
		defer client.Logout()

		if err := bmc.ResetManager(client, settingsPreserveConfig); err != nil {
			return fmt.Errorf("failed to reset manager: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Manager reset initiated (preserve-config: %s)\n", settingsPreserveConfig)
		return nil
	},
}

func init() {
	SettingsGetCmd.Flags().VarP(&settingsFormat, "output-format", "F", "Set the output format (json|yaml).")
	SettingsResetCmd.Flags().StringVar(&settingsPreserveConfig, "preserve-config", "", "Preserve settings during reset (PreserveNetwork|PreserveNetworkAndUsers).")

	// Common flags for commands that connect to BMC
	for _, c := range []*cobra.Command{SettingsListCmd, SettingsGetCmd, SettingsSetCmd, SettingsResetCmd} {
		c.Flags().StringVarP(&settingsInventoryFile, "inventory-file", "f", "", "File containing node inventory.")
		c.Flags().Var(&settingsInputFormat, "input-format", "Set the inventory input format (json|yaml).")
		c.Flags().StringVarP(&bmc.SettingsUsername, "username", "u", "", "Set the master BMC username.")
		c.Flags().StringVarP(&bmc.SettingsPassword, "password", "p", "", "Set the master BMC password.")
		c.Flags().StringVar(&bmc.SettingsSecretsFile, "secrets-file", "secrets.json", "Set the secrets file with BMC credentials.")
		c.Flags().BoolVarP(&bmc.SettingsInsecure, "insecure", "i", false, "Skip TLS certificate verification during probe.")
		c.Flags().StringVar(&bmc.SettingsCACertPath, "cacert", "", "Set the path to CA cert file (defaults to system CAs when blank).")
	}

	SettingsCmd.AddCommand(SettingsListCmd)
	SettingsCmd.AddCommand(SettingsGetCmd)
	SettingsCmd.AddCommand(SettingsSetCmd)
	SettingsCmd.AddCommand(SettingsResetCmd)

	rootCmd.AddCommand(SettingsCmd)
}
