package cmd

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/openchami/magellan/internal/format"
	"github.com/openchami/magellan/pkg/bmc"
	"github.com/openchami/magellan/pkg/crawler"
	"github.com/openchami/magellan/pkg/power"
	"github.com/openchami/magellan/pkg/secrets"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stmcginnis/gofish"
)

var settingsFormat format.DataFormat = format.FORMAT_JSON

// Category names used by settings subcommands.
var settingsCategories = map[string]string{
	"NetworkProtocol":   "Network service settings (SSH, HTTPS, IPMI, NTP, etc.)",
	"EthernetInterface": "Network interface settings (IP, MAC, DHCP, etc.)",
	"ComputerSystem":    "System-level settings (boot order, asset tag, etc.)",
	"Manager":           "Manager properties (firmware version, model, etc.)",
	"Accounts":          "BMC user accounts (username, role, etc.)",
	"Reset":             "Factory reset the BMC manager",
}

var SettingsCmd = &cobra.Command{
	Use: "settings",
	Example: `  # list all available setting categories
  magellan settings list

  # list properties in the NetworkProtocol category
  magellan settings list NetworkProtocol

  # get a specific network setting from a BMC
  magellan settings get 172.16.0.105 NetworkProtocol SSH

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
	Use:   "list [category]",
	Short: "List available setting categories or properties",
	Long: `List available setting categories or properties within a category.

When called without arguments, lists all available categories.
When called with a category name, lists the properties available in that category.`,
	Example: `  # list all available categories
  magellan settings list

  # list properties in NetworkProtocol
  magellan settings list NetworkProtocol`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Available setting categories:")
			fmt.Println()
			for name, desc := range settingsCategories {
				fmt.Printf("  %-20s %s\n", name, desc)
			}
			return
		}
		category := args[0]
		if _, ok := settingsCategories[category]; !ok {
			log.Error().Msgf("unknown category %q; use 'magellan settings list' to see available categories", category)
			os.Exit(1)
		}
		fmt.Printf("Category: %s\n", category)
		fmt.Printf("Description: %s\n", settingsCategories[category])
		fmt.Println()
		switch category {
		case "NetworkProtocol":
			fmt.Println("  DHCP        DHCPv4 protocol settings")
			fmt.Println("  DHCPv6      DHCPv6 protocol settings")
			fmt.Println("  FQDN        Fully qualified domain name")
			fmt.Println("  FTP         File Transfer Protocol settings")
			fmt.Println("  FTPS        FTP over SSL settings")
			fmt.Println("  HTTP        HTTP protocol settings")
			fmt.Println("  HTTPS       HTTPS/SSL protocol settings")
			fmt.Println("  HostName    Host name without domain")
			fmt.Println("  IPMI        IPMI over LAN settings")
			fmt.Println("  KVMIP       KVM-IP settings")
			fmt.Println("  NTP         NTP protocol settings")
			fmt.Println("  Proxy       HTTP/HTTPS proxy configuration")
			fmt.Println("  RDP         Remote Desktop Protocol settings")
			fmt.Println("  RFB         Remote Frame Buffer settings")
			fmt.Println("  SFTP        SFTP settings")
			fmt.Println("  SNMP        SNMP settings")
			fmt.Println("  SSDP        SSDP settings")
			fmt.Println("  SSH         Secure Shell settings")
			fmt.Println("  Telnet      Telnet settings")
			fmt.Println("  VirtualMedia Virtual Media settings")
			fmt.Println("  MDNS        Multicast DNS settings")
		case "EthernetInterface":
			fmt.Println("  0           First ethernet interface")
			fmt.Println("  1           Second ethernet interface (if present)")
			fmt.Println("  ...         Additional interfaces")
		case "ComputerSystem":
			fmt.Println("  Boot        Boot settings and order")
			fmt.Println("  BiosVersion BIOS version string")
			fmt.Println("  AssetTag    Asset tag identifier")
			fmt.Println("  Manufacturer Manufacturer name")
			fmt.Println("  Model       System model")
			fmt.Println("  SerialNumber Serial number")
			fmt.Println("  UUID        System UUID")
		case "Manager":
			fmt.Println("  ID          Manager identifier")
			fmt.Println("  Name        Manager name")
			fmt.Println("  FirmwareVersion Firmware version")
			fmt.Println("  ManagerType Manager type")
			fmt.Println("  Model       Manager model")
			fmt.Println("  SerialNumber Serial number")
		case "Accounts":
			fmt.Println("  (use 'get' to query specific accounts)")
		case "Reset":
			fmt.Println("  ResetType   ResetAll, PreserveNetwork, or PreserveNetworkAndUsers")
		}
	},
}

var SettingsGetCmd = &cobra.Command{
	Use:   "get <node> <category> [property]",
	Short: "Get a BMC setting value",
	Long: `Get a BMC setting value by category and property.

For NetworkProtocol, specify the protocol name (e.g., SSH, HTTPS, IPMI, NTP).
For EthernetInterface, specify the interface index (0, 1, ...).
For ComputerSystem, specify the property name.
For Manager, specify the property name or ID.
For Accounts, specify the account ID.`,
	Example: `  # get SSH protocol settings
  magellan settings get 172.16.0.105 NetworkProtocol SSH

  # get the first ethernet interface
  magellan settings get 172.16.0.105 EthernetInterface 0

  # get boot settings
  magellan settings get 172.16.0.105 ComputerSystem Boot

  # get all accounts
  magellan settings get 172.16.0.105 Accounts

  # get a specific account
  magellan settings get 172.16.0.105 Accounts 1`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 2 {
			log.Error().Msg("usage: magellan settings get <node> <category> [property]")
			os.Exit(1)
		}
		nodeArg := args[0]
		category := args[1]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			log.Error().Err(err).Msgf("failed to connect to BMC for node %q", nodeArg)
			os.Exit(1)
		}
		defer power.LogoutBMCSessions()

		var result any
		switch category {
		case "NetworkProtocol":
			np, err := bmc.GetNetworkProtocol(client)
			if err != nil {
				log.Error().Err(err).Msg("failed to get network protocol")
				os.Exit(1)
			}
			if len(args) > 2 {
				proto := args[2]
				field := reflect.ValueOf(np).Elem().FieldByName(proto)
				if !field.IsValid() {
					log.Error().Msgf("unknown protocol %q", proto)
					os.Exit(1)
				}
				result = field.Interface()
			} else {
				result = np
			}
		case "EthernetInterface":
			ifaces, err := bmc.GetEthernetInterfaces(client)
			if err != nil {
				log.Error().Err(err).Msg("failed to get ethernet interfaces")
				os.Exit(1)
			}
			if len(args) > 2 {
				idx := 0
				if _, err := fmt.Sscanf(args[2], "%d", &idx); err != nil {
					log.Error().Err(err).Msgf("invalid interface index %q", args[2])
					os.Exit(1)
				}
				if idx < 0 || idx >= len(ifaces) {
					log.Error().Msgf("interface index %d out of range (0-%d)", idx, len(ifaces)-1)
					os.Exit(1)
				}
				result = ifaces[idx]
			} else {
				result = ifaces
			}
		case "ComputerSystem":
			sys, err := bmc.GetComputerSystem(client, "1")
			if err != nil {
				log.Error().Err(err).Msg("failed to get computer system")
				os.Exit(1)
			}
			if len(args) > 2 {
				field := reflect.ValueOf(sys).Elem().FieldByName(args[2])
				if !field.IsValid() {
					log.Error().Msgf("unknown property %q on ComputerSystem", args[2])
					os.Exit(1)
				}
				result = field.Interface()
			} else {
				result = sys
			}
		case "Manager":
			service := client.GetService()
			managers, err := service.Managers()
			if err != nil {
				log.Error().Err(err).Msg("failed to list managers")
				os.Exit(1)
			}
			if len(args) > 2 {
				found := false
				for _, mgr := range managers {
					if mgr.ID == args[2] || mgr.Name == args[2] {
						result = mgr
						found = true
						break
					}
				}
				if !found {
					log.Error().Msgf("manager %q not found", args[2])
					os.Exit(1)
				}
			} else {
				result = managers
			}
		case "Accounts":
			accts, err := bmc.ListAccounts(client)
			if err != nil {
				log.Error().Err(err).Msg("failed to list accounts")
				os.Exit(1)
			}
			if len(args) > 2 {
				found := false
				for _, acct := range accts {
					if acct.ID == args[2] {
						result = acct
						found = true
						break
					}
				}
				if !found {
					log.Error().Msgf("account %q not found", args[2])
					os.Exit(1)
				}
			} else {
				result = accts
			}
		default:
			log.Error().Msgf("unknown category %q; use 'magellan settings list' to see available categories", category)
			os.Exit(1)
		}

		output, err := format.MarshalData(result, settingsFormat)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal result")
			os.Exit(1)
		}
		fmt.Println(string(output))
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
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 4 {
			log.Error().Msg("usage: magellan settings set <node> <category> <property> <value>")
			os.Exit(1)
		}
		nodeArg := args[0]
		category := args[1]
		property := args[2]
		value := args[3]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			log.Error().Err(err).Msgf("failed to connect to BMC for node %q", nodeArg)
			os.Exit(1)
		}
		defer power.LogoutBMCSessions()

		switch category {
		case "NetworkProtocol":
			if err := bmc.SetNetworkProtocol(client, property, value); err != nil {
				log.Error().Err(err).Msgf("failed to set NetworkProtocol.%s", property)
				os.Exit(1)
			}
		case "EthernetInterface":
			idx := 0
			if _, err := fmt.Sscanf(property, "%d", &idx); err != nil {
				log.Error().Err(err).Msgf("invalid interface index %q", property)
				os.Exit(1)
			}
			if err := bmc.SetEthernetInterface(client, idx, value); err != nil {
				log.Error().Err(err).Msgf("failed to set EthernetInterface[%d]", idx)
				os.Exit(1)
			}
		case "ComputerSystem":
			if err := bmc.SetComputerSystem(client, property, value); err != nil {
				log.Error().Err(err).Msgf("failed to set ComputerSystem.%s", property)
				os.Exit(1)
			}
		case "Manager":
			if err := bmc.SetManager(client, property, value); err != nil {
				log.Error().Err(err).Msgf("failed to set Manager.%s", property)
				os.Exit(1)
			}
		case "Accounts":
			if err := bmc.UpdateAccount(client, property, value); err != nil {
				log.Error().Err(err).Msgf("failed to update account %s", property)
				os.Exit(1)
			}
		default:
			log.Error().Msgf("unknown category %q; use 'magellan settings list' to see available categories", category)
			os.Exit(1)
		}

		fmt.Printf("Successfully set %s.%s\n", category, property)
	},
}

var settingsPreserveConfig string

var SettingsResetCmd = &cobra.Command{
	Use:   "reset <node>",
	Short: "Factory reset the BMC manager",
	Long: `Factory reset the BMC manager.

By default, this resets all settings. Use --preserve-config to preserve
specific settings during the reset.`,
	Example: `  # factory reset all settings
  magellan settings reset 172.16.0.105

  # factory reset but preserve network settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetwork

  # factory reset but preserve network and user settings
  magellan settings reset 172.16.0.105 --preserve-config PreserveNetworkAndUsers`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Error().Msg("usage: magellan settings reset <node>")
			os.Exit(1)
		}
		nodeArg := args[0]

		client, err := settingsConnect(nodeArg)
		if err != nil {
			log.Error().Err(err).Msgf("failed to connect to BMC for node %q", nodeArg)
			os.Exit(1)
		}
		defer power.LogoutBMCSessions()

		service := client.GetService()
		managers, err := service.Managers()
		if err != nil {
			log.Error().Err(err).Msg("failed to list managers")
			os.Exit(1)
		}
		if len(managers) == 0 {
			log.Error().Msg("no managers found on BMC")
			os.Exit(1)
		}

		if err := bmc.ResetManager(client, settingsPreserveConfig); err != nil {
			log.Error().Err(err).Msg("failed to reset manager")
			os.Exit(1)
		}
		fmt.Printf("Manager reset initiated (preserve-config: %s)\n", settingsPreserveConfig)
	},
}

// settingsConnect resolves a node argument to a BMC connection.
// If the argument is an IP address or hostname, it connects directly.
// Otherwise, it looks up the node in the inventory to find the BMC address.
func settingsConnect(nodeArg string) (*gofish.APIClient, error) {
	// If it looks like a direct IP or hostname, connect directly
	if isIPAddressOrHostname(nodeArg) {
		var store secrets.SecretStore
		if username != "" && password != "" {
			store = secrets.NewStaticStore(username, password)
		} else if secretsFile != "" {
			var err error
			store, err = secrets.OpenStore(secretsFile)
			if err != nil {
				return nil, fmt.Errorf("failed to open secrets file: %w", err)
			}
		}
		config := crawler.CrawlerConfig{
			URI:             "https://" + nodeArg,
			CredentialStore: store,
			Insecure:        insecure,
		}
		return crawler.GetBMCClient(config)
	}

	// Resolve from inventory
	var datafile string
	if viper.IsSet("inventory-file") {
		datafile = viper.GetString("inventory-file")
	} else {
		datafile = viper.GetString("collect.output-file")
		if datafile != "" {
			log.Info().Msgf("parsing default inventory file from 'collect': %s", datafile)
		}
	}
	if datafile == "" {
		return nil, fmt.Errorf("node %q is not an IP/hostname and no inventory file found; use --inventory-file or set collect.output-file", nodeArg)
	}

	nodes, err := power.ParseInventory(datafile, settingsFormat)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inventory file %s: %w", datafile, err)
	}

	// Simple lookup by ClusterID (xname)
	var found *bmc.Node
	for i := range nodes {
		if nodes[i].ClusterID == nodeArg || nodes[i].NodeID == nodeArg {
			found = &nodes[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("node %q not found in inventory", nodeArg)
	}

	// Build credential store
	var store secrets.SecretStore
	if username != "" && password != "" {
		store = secrets.NewStaticStore(username, password)
	} else if secretsFile != "" {
		store, err = secrets.OpenStore(secretsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open secrets file: %w", err)
		}
	}

	config := crawler.CrawlerConfig{
		URI:             "https://" + found.BmcIP,
		CredentialStore: store,
		Insecure:        insecure,
	}
	return crawler.GetBMCClient(config)
}

// isIPAddressOrHostname checks if a string looks like an IP address or hostname.
func isIPAddressOrHostname(s string) bool {
	// Check for IPv4
	parts := strings.Split(s, ".")
	if len(parts) == 4 {
		allNumeric := true
		for _, p := range parts {
			if len(p) == 0 {
				allNumeric = false
				break
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					allNumeric = false
					break
				}
			}
			if !allNumeric {
				break
			}
		}
		if allNumeric {
			return true
		}
	}
	// Check for IPv6 (contains colons)
	if strings.Contains(s, ":") {
		return true
	}
	// Check for hostname (contains dots or ends with .local)
	if strings.Contains(s, ".") || strings.HasSuffix(s, ".local") {
		return true
	}
	return false
}

func init() {
	SettingsGetCmd.Flags().VarP(&settingsFormat, "output-format", "F", "Set the output format (json|yaml).")
	SettingsResetCmd.Flags().StringVar(&settingsPreserveConfig, "preserve-config", "", "Preserve settings during reset (PreserveNetwork|PreserveNetworkAndUsers).")

	// Common flags for commands that connect to BMC
	for _, c := range []*cobra.Command{SettingsGetCmd, SettingsSetCmd, SettingsResetCmd} {
		c.Flags().StringP("inventory-file", "f", "", "YAML file containing node inventory.")
		c.Flags().StringVarP(&username, "username", "u", "", "Set the master BMC username.")
		c.Flags().StringVarP(&password, "password", "p", "", "Set the master BMC password.")
		c.Flags().StringVar(&secretsFile, "secrets-file", "", "Set the secrets file with BMC credentials.")
		c.Flags().BoolVarP(&insecure, "insecure", "i", false, "Skip TLS certificate verification during probe.")
		c.Flags().String("cacert", "", "Set the path to CA cert file (defaults to system CAs when blank).")
	}

	SettingsCmd.AddCommand(SettingsListCmd)
	SettingsCmd.AddCommand(SettingsGetCmd)
	SettingsCmd.AddCommand(SettingsSetCmd)
	SettingsCmd.AddCommand(SettingsResetCmd)

	rootCmd.AddCommand(SettingsCmd)
}
